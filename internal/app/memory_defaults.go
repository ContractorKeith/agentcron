package app

import (
	"github.com/ContractorKeith/agentcron/internal/config"
	"github.com/ContractorKeith/agentcron/internal/memory"
)

// DefaultMemory enables new jobs only when their configured mdkb is present.
func DefaultMemory(opts Options) (bool, error) {
	cfg, err := config.Load(opts.configOptions())
	if err != nil {
		return false, err
	}
	_, err = memory.Resolve(cfg.MDKBPath, cfg.MDKBRoot)
	return err == nil, nil
}
