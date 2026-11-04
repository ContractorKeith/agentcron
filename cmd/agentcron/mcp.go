package main

import (
	"fmt"

	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/ContractorKeith/agentcron/internal/mcpserver"
	"github.com/spf13/cobra"
)

func mcpCommands(root *cobra.Command, opts *app.Options, output outputFunc) {
	var writes bool
	cmd := &cobra.Command{Use: "mcp", Short: "Serve MCP over stdio", Args: func(_ *cobra.Command, args []string) error {
		if len(args) != 0 {
			return &app.Error{Code: app.Usage, Err: fmt.Errorf("no arguments expected")}
		}
		return nil
	}, RunE: func(cmd *cobra.Command, _ []string) error {
		if opts.ReadOnly {
			return &app.Error{Code: app.Usage, Err: fmt.Errorf("MCP does not support --now previews")}
		}
		return mcpserver.Run(cmd.Context(), *opts, writes, version)
	}}
	cmd.Flags().BoolVar(&writes, "allow-write", false, "Register write tools (human approval still required)")
	root.AddCommand(cmd, approveCommand(opts, output))
}
