package app

import (
	"github.com/ContractorKeith/agentcron/internal/config"
	"github.com/ContractorKeith/agentcron/internal/daemon"
)

func QueueMessage(opts Options) string {
	cfg, err := config.Load(opts.configOptions())
	if err == nil && daemon.Holder(cfg.DataDir) != 0 {
		return "queued"
	}
	return "queued; no daemon running"
}
