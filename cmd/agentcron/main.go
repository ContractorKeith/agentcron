package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	if err := execute(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "agentcron:", err)
		os.Exit(int(app.Code(err)))
	}
}

func newRoot() *cobra.Command {
	var opts app.Options
	var jsonOutput, quiet bool
	root := &cobra.Command{
		Use: "agentcron", Short: "A local scheduler for recurring agent work",
		SilenceUsage: true, SilenceErrors: true,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	root.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Print JSON")
	root.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress success output (except JSON)")
	root.PersistentFlags().StringVar(&opts.ConfigDir, "config", "", "Config directory (flag > AGENTCRON_CONFIG_DIR > XDG)")
	root.PersistentFlags().StringVar(&opts.DataDir, "data-dir", "", "Data directory (flag > AGENTCRON_DATA_DIR > file > XDG)")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &app.Error{Code: app.Usage, Err: err} })
	output := func(cmd *cobra.Command, value any, format string, args ...any) error {
		if jsonOutput {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(value)
		}
		if quiet {
			return nil
		}
		_, err := fmt.Fprintf(cmd.OutOrStdout(), format, args...)
		return err
	}
	noArgs := func(cmd *cobra.Command, args []string) error {
		if err := cobra.NoArgs(cmd, args); err != nil {
			return &app.Error{Code: app.Usage, Err: err}
		}
		return nil
	}
	root.Args = noArgs
	root.AddCommand(&cobra.Command{
		Use: "version", Short: "Print the build version", Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return output(cmd, struct {
				Version string `json:"version"`
			}{version}, "%s\n", version)
		},
	})
	root.AddCommand(&cobra.Command{
		Use: "init", Short: "Initialise configuration, jobs and the ledger", Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := app.Init(cmd.Context(), opts); err != nil {
				return err
			}
			return output(cmd, struct {
				Initialized bool `json:"initialized"`
			}{true}, "Initialised agentcron\n")
		},
	})
	root.AddCommand(&cobra.Command{
		Use: "doctor", Short: "Inspect directories, ledger and runner availability", Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			health, err := app.Doctor(cmd.Context(), opts)
			if err != nil && app.Code(err) != app.NotInitialized {
				return err
			}
			if outErr := output(cmd, health,
				"Config: %s (%t)\nData: %s (%t)\nSchema version: %d\ngit: %t\nclaude: %t\ncodex: %t\nmdkb: %t\nmdkb version: %s\nKB root: %s\nKB initialised: %t\nService installed: %t active: %t\nDaemon pid: %d\n",
				health.ConfigPath, health.ConfigDir, health.DataPath, health.DataDir, health.SchemaVersion,
				health.Git, health.Claude, health.Codex, health.MDKB, health.MDKBVersion, health.KBRoot, health.KBInitialised, health.Service.Installed, health.Service.Active, health.DaemonPID); outErr != nil {
				return outErr
			}
			return err
		},
	})
	jobCommands(root, &opts, output)
	runCommands(root, &opts, output)
	daemonCommands(root, &opts)
	serviceCommands(root, &opts, output)
	memoryCommand(root, &opts, output)
	return root
}

func execute(args []string) error {
	root := newRoot()
	if _, _, err := root.Find(args); err != nil {
		return &app.Error{Code: app.Usage, Err: err}
	}
	root.SetArgs(args)
	return root.Execute()
}
