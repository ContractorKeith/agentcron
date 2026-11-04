package main

import (
	"fmt"

	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/spf13/cobra"
)

func memoryCommand(root *cobra.Command, opts *app.Options, output outputFunc) {
	root.AddCommand(&cobra.Command{
		Use: "memory <job>", Short: "Print the job's mdkb memory note", Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return &app.Error{Code: app.Usage, Err: fmt.Errorf("exactly one job name required")}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			note, err := app.Memory(cmd.Context(), *opts, args[0])
			if err != nil {
				return err
			}
			return output(cmd, struct {
				Note string `json:"note"`
			}{note}, "%s", note)
		},
	})
}
