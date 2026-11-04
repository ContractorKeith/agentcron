package main

import (
	"fmt"

	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/spf13/cobra"
)

func approveCommand(opts *app.Options, output outputFunc) *cobra.Command {
	return &cobra.Command{Use: "approve <job>", Short: "Approve and resume an agent-created job", Args: func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return &app.Error{Code: app.Usage, Err: fmt.Errorf("exactly one job name required")}
		}
		return nil
	}, RunE: func(cmd *cobra.Command, args []string) error {
		if err := app.ApproveJob(cmd.Context(), *opts, clock.Real{}, args[0]); err != nil {
			return err
		}
		return output(cmd, nil, "Approved %s\n", args[0])
	}}
}
