package main

import (
	"fmt"

	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/spf13/cobra"
)

func addJobCommand(root *cobra.Command, opts *app.Options, output outputFunc, current func() clock.Clock, args cobra.PositionalArgs) {
	j := app.NewJob("")
	var dry, noMemory bool
	cmd := &cobra.Command{Use: "add <name>", Short: "Add a job definition", Args: args, RunE: func(cmd *cobra.Command, args []string) error {
		j.Name = args[0]
		if cmd.Flags().Changed("no-memory") {
			j.Memory = !noMemory
		}
		if !cmd.Flags().Changed("memory") && !cmd.Flags().Changed("no-memory") {
			var err error
			j.Memory, err = app.DefaultMemory(*opts)
			if err != nil {
				return err
			}
		}
		text, err := app.AddJob(cmd.Context(), *opts, current(), j, dry)
		if err != nil {
			return err
		}
		if dry {
			_, err = fmt.Fprint(cmd.OutOrStdout(), text)
			return err
		}
		return output(cmd, j, "Added %s\n", j.Name)
	}}
	f := cmd.Flags()
	f.StringVar(&j.Catchup, "catchup", "one", "Missed fires: one or none")
	f.StringVar(&j.Schedule, "schedule", "", "Schedule expression")
	f.StringVar(&j.Runner, "runner", "", "claude, codex or command")
	f.StringVar(&j.Prompt, "prompt", "", "Prompt text")
	f.StringVar(&j.PromptFile, "prompt-file", "", "Prompt file relative to jobs directory")
	f.StringVar(&j.Repo, "repo", "", "Repository")
	f.StringVar(&j.Base, "base", "", "Base ref (requires repo)")
	f.StringVar(&j.Timezone, "timezone", "", "IANA timezone (default local)")
	f.StringArrayVar(&j.Args, "arg", nil, "Runner argument (repeatable)")
	f.StringArrayVar(&j.Command, "command", nil, "Command argv element (repeatable)")
	f.StringVar(&j.Timeout, "timeout", "30m", "Run timeout")
	f.StringVar(&j.Expect, "expect", "", "Expected stdout regex")
	f.BoolVar(&j.Memory, "memory", false, "Enable memory (default: mdkb presence)")
	f.BoolVar(&noMemory, "no-memory", false, "Disable memory")
	f.BoolVar(&dry, "dry-run", false, "Print TOML without writing")
	cmd.MarkFlagsMutuallyExclusive("memory", "no-memory")
	root.AddCommand(cmd)
}
