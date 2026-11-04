package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/spf13/cobra"
)

type outputFunc func(*cobra.Command, any, string, ...any) error

func jobCommands(root *cobra.Command, opts *app.Options, output outputFunc) {
	var now string
	root.PersistentFlags().StringVar(&now, "now", "", "")
	// Test-only clock pin for reproducible acceptance goldens; never used by normal runs.
	_ = root.PersistentFlags().MarkHidden("now")
	current := func() clock.Clock {
		if now != "" {
			t, _ := time.Parse(time.RFC3339, now)
			return clock.NewFake(t)
		}
		return clock.Real{}
	}
	root.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		if now != "" {
			if _, err := time.Parse(time.RFC3339, now); err != nil {
				return &app.Error{Code: app.Usage, Err: fmt.Errorf("now: %w", err)}
			}
		}
		return nil
	}
	one := func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return &app.Error{Code: app.Usage, Err: fmt.Errorf("exactly one job name required")}
		}
		return nil
	}
	none := func(_ *cobra.Command, args []string) error {
		if len(args) != 0 {
			return &app.Error{Code: app.Usage, Err: fmt.Errorf("no arguments expected")}
		}
		return nil
	}
	addJobCommand(root, opts, output, current, one)
	root.AddCommand(&cobra.Command{Use: "ls", Short: "List job definitions and state", Args: none, RunE: func(cmd *cobra.Command, _ []string) error {
		views, err := app.ListJobs(cmd.Context(), *opts, current())
		if err != nil {
			return err
		}
		data, _ := json.MarshalIndent(views, "", "  ")
		return output(cmd, views, "%s\n", data)
	}})
	root.AddCommand(&cobra.Command{Use: "show <name>", Short: "Show a job and its next five fires", Args: one, RunE: func(cmd *cobra.Command, args []string) error {
		detail, err := app.ShowJob(cmd.Context(), *opts, current(), args[0])
		if err != nil {
			return err
		}
		data, _ := json.MarshalIndent(detail, "", "  ")
		return output(cmd, detail, "%s\n", data)
	}})
	var dry bool
	remove := &cobra.Command{Use: "rm <name>", Short: "Remove a job definition", Args: one, RunE: func(cmd *cobra.Command, args []string) error {
		if err := app.RemoveJob(cmd.Context(), *opts, current(), args[0], dry); err != nil {
			return err
		}
		return output(cmd, nil, "Removed %s (dry-run=%t)\n", args[0], dry)
	}}
	remove.Flags().BoolVar(&dry, "dry-run", false, "Preview removal")
	root.AddCommand(remove)
	for _, paused := range []bool{true, false} {
		name := "resume"
		if paused {
			name = "pause"
		}
		root.AddCommand(&cobra.Command{Use: name + " <name>", Short: name + " a job", Args: one, RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.PauseJob(cmd.Context(), *opts, current(), args[0], paused); err != nil {
				return err
			}
			return output(cmd, nil, "%s: paused=%t\n", args[0], paused)
		}})
	}
	var expr, zone string
	var count int
	next := &cobra.Command{Use: "next", Short: "Show upcoming fires", Args: none, RunE: func(cmd *cobra.Command, _ []string) error {
		result, err := app.NextFires(cmd.Context(), *opts, current(), expr, zone, count)
		if err != nil {
			return err
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return output(cmd, result, "%s\n", data)
	}}
	next.Flags().StringVar(&expr, "schedule", "", "Ad hoc schedule")
	next.Flags().StringVar(&zone, "timezone", "", "Schedule timezone")
	next.Flags().IntVar(&count, "count", 5, "Fires per job (1–1000)")
	root.AddCommand(next)
}
