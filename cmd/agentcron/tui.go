package main

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/tui"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

func tuiCommand(opts *app.Options) *cobra.Command {
	return &cobra.Command{
		Use: "tui", Short: "Browse jobs, runs and live output", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out, ok := cmd.OutOrStdout().(*os.File)
			if !ok || !isatty.IsTerminal(out.Fd()) {
				return &app.Error{Code: app.Usage, Err: fmt.Errorf("tui requires stdout to be a terminal")}
			}
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			c := clock.Real{}
			_, err := tea.NewProgram(tui.New(ctx, app.TUI{Options: *opts, Clock: c}, c), tea.WithContext(ctx), tea.WithOutput(out), tea.WithInput(cmd.InOrStdin())).Run()
			return err
		},
	}
}
