package main

import (
	"encoding/json"

	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/spf13/cobra"
)

func serviceCommands(root *cobra.Command, opts *app.Options, output outputFunc) {
	parent := &cobra.Command{Use: "service", Short: "Manage the user service", Args: cobra.NoArgs}
	for _, action := range []string{"install", "uninstall", "status"} {
		var dry bool
		var target string
		cmd := &cobra.Command{Use: action, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := app.Service(cmd.Context(), *opts, action, dry, target)
			if err != nil {
				return err
			}
			b, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			return output(cmd, result, "%s\n", b)
		}}
		cmd.Flags().BoolVar(&dry, "dry-run", false, "Preview unit and commands")
		cmd.Flags().StringVar(&target, "os", "", "Target OS for dry-run")
		_ = cmd.Flags().MarkHidden("os")
		parent.AddCommand(cmd)
	}
	root.AddCommand(parent)
}
