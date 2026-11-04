package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/spf13/cobra"
)

func runCommands(root *cobra.Command, opts *app.Options, output outputFunc) {
	current := clock.Real{}
	one := func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return &app.Error{Code: app.Usage, Err: fmt.Errorf("exactly one argument required")}
		}
		return nil
	}
	atMostOne := func(_ *cobra.Command, args []string) error {
		if len(args) > 1 {
			return &app.Error{Code: app.Usage, Err: fmt.Errorf("at most one argument expected")}
		}
		return nil
	}
	none := func(_ *cobra.Command, args []string) error {
		if len(args) > 0 {
			return &app.Error{Code: app.Usage, Err: fmt.Errorf("no arguments expected")}
		}
		return nil
	}
	printJSON := func(cmd *cobra.Command, v any) error {
		b, e := json.MarshalIndent(v, "", "  ")
		if e != nil {
			return e
		}
		return output(cmd, v, "%s\n", b)
	}
	var now, wait bool
	run := &cobra.Command{Use: "run <job>", Short: "Enqueue or run a job inline", Args: one, RunE: func(cmd *cobra.Command, args []string) error {
		r, err := app.RunJob(cmd.Context(), *opts, current, args[0], now, wait)
		if r.ID != "" {
			if e := output(cmd, r, "%s\n", r.ID); e != nil {
				return e
			}
		}
		return err
	}}
	run.Flags().BoolVar(&now, "now", false, "Run inline and wait for completion")
	run.Flags().BoolVar(&wait, "wait", false, "Wait for the run's terminal state")
	root.AddCommand(run)
	var state string
	var limit int
	runs := &cobra.Command{Use: "runs [job]", Short: "List runs", Args: atMostOne, RunE: func(cmd *cobra.Command, args []string) error {
		job := ""
		if len(args) > 0 {
			job = args[0]
		}
		r, e := app.Runs(cmd.Context(), *opts, current, job, state, limit)
		if e != nil {
			return e
		}
		return printJSON(cmd, r)
	}}
	runs.Flags().StringVar(&state, "state", "", "Filter by run state")
	runs.Flags().IntVar(&limit, "limit", 50, "Maximum runs (1–10000)")
	root.AddCommand(runs)
	var stderr, follow bool
	logs := &cobra.Command{Use: "logs <run-id>", Short: "Read run output", Args: one, RunE: func(cmd *cobra.Command, args []string) error {
		jsonOutput, _ := cmd.Flags().GetBool("json")
		if jsonOutput {
			if follow {
				return &app.Error{Code: app.Usage, Err: fmt.Errorf("--json cannot be combined with --follow")}
			}
			var b bytes.Buffer
			if e := app.Logs(cmd.Context(), *opts, current, args[0], stderr, false, &b); e != nil {
				return e
			}
			return output(cmd, struct {
				Output string `json:"output"`
			}{b.String()}, "%s", b.String())
		}
		return app.Logs(cmd.Context(), *opts, current, args[0], stderr, follow, cmd.OutOrStdout())
	}}
	logs.Flags().BoolVar(&stderr, "stderr", false, "Read stderr")
	logs.Flags().BoolVar(&follow, "follow", false, "Follow until terminal")
	root.AddCommand(logs)
	root.AddCommand(&cobra.Command{Use: "cancel <run-id>", Short: "Cancel a running process group", Args: one, RunE: func(cmd *cobra.Command, args []string) error {
		return app.CancelRun(cmd.Context(), *opts, current, args[0])
	}})
	root.AddCommand(&cobra.Command{Use: "worktrees", Short: "List active and kept run directories", Args: none, RunE: func(cmd *cobra.Command, _ []string) error {
		r, e := app.Worktrees(cmd.Context(), *opts, current)
		if e != nil {
			return e
		}
		return printJSON(cmd, r)
	}})
	var dry, force bool
	prune := &cobra.Command{Use: "prune", Short: "Remove kept directories of terminal runs", Args: none, RunE: func(cmd *cobra.Command, _ []string) error {
		r, e := app.Prune(cmd.Context(), *opts, current, dry, force)
		if e != nil {
			return e
		}
		return printJSON(cmd, r)
	}}
	prune.Flags().BoolVar(&force, "force", false, "Destroy dirty files and commits unique to run branches")
	prune.Flags().BoolVar(&dry, "dry-run", false, "Preview removals")
	root.AddCommand(prune)
}
