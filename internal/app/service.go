package app

import (
	"context"
	"fmt"
	"os"

	"github.com/ContractorKeith/agentcron/internal/config"
	"github.com/ContractorKeith/agentcron/internal/service"
)

func serviceOptions(opts Options) (service.Options, error) {
	cfg, err := config.Load(opts.configOptions())
	if err != nil {
		return service.Options{}, err
	}
	binary, err := os.Executable()
	if err != nil {
		return service.Options{}, err
	}
	return service.Current(binary, os.Getenv("PATH"), cfg.ConfigDir, cfg.DataDir)
}
func Service(ctx context.Context, opts Options, action string, dry bool, target string) (any, error) {
	if opts.ReadOnly {
		return nil, &Error{Usage, fmt.Errorf("now preview is read-only")}
	}
	o, err := serviceOptions(opts)
	if err != nil {
		return nil, err
	}
	if target != "" {
		if !dry {
			return nil, &Error{Usage, fmt.Errorf("--os requires --dry-run")}
		}
		o.OS = target
	}
	if action == "status" {
		return service.Status(ctx, o, service.Exec{})
	}
	return service.Apply(ctx, o, action, dry, service.Exec{})
}
