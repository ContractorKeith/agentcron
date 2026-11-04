package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/spf13/cobra"
)

func daemonCommands(root *cobra.Command, opts *app.Options) {
	for _, loop := range []bool{true, false} {
		name := "tick"
		if loop {
			name = "daemon"
		}
		var noWait bool
		cmd := &cobra.Command{Use: name, Short: "Run the scheduler", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			signals := make(chan os.Signal, 1)
			signal.Notify(signals, syscall.SIGHUP)
			defer signal.Stop(signals)
			hup := make(chan struct{}, 1)
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-signals:
						select {
						case hup <- struct{}{}:
						default:
						}
					}
				}
			}()
			return app.Schedule(ctx, *opts, clock.Real{}, loop, noWait, hup, cmd.ErrOrStderr())
		}}
		if !loop {
			cmd.Flags().BoolVar(&noWait, "no-wait", false, "Return after launching a detached tick worker")
		}
		root.AddCommand(cmd)
	}
}
