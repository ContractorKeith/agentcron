package app

import (
	"context"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/config"
	"github.com/ContractorKeith/agentcron/internal/daemon"
	"github.com/ContractorKeith/agentcron/internal/jobs"
)

// TUI exposes the interactive use cases without exposing storage or runner internals.
type TUI struct {
	Options Options
	Clock   clock.Clock
}

func (a TUI) Jobs(ctx context.Context) ([]JobView, error) {
	opts := a.Options
	opts.ReadOnly = true
	views, err := ListJobs(ctx, opts, a.Clock)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(opts.configOptions())
	if err != nil {
		return nil, err
	}
	for i := range views {
		if j := views[i].Job; j != nil && j.PromptFile != "" {
			prompt, err := jobs.ReadPrompt(cfg.JobsDir(), *j)
			if err != nil {
				views[i].Error = err.Error()
			} else {
				j.Prompt = prompt
			}
		}
	}
	return views, nil
}
func (a TUI) Runs(ctx context.Context, name string) ([]Run, error) {
	return Runs(ctx, a.Options, a.Clock, name, "", 100)
}
func (a TUI) Enqueue(ctx context.Context, name string) error {
	_, err := RunJob(ctx, a.Options, a.Clock, name, false, false)
	return err
}
func (a TUI) Pause(ctx context.Context, name string, paused bool) error {
	return PauseJob(ctx, a.Options, a.Clock, name, paused)
}
func (a TUI) Cancel(ctx context.Context, id string) error {
	return CancelRun(ctx, a.Options, a.Clock, id)
}
func (a TUI) Tail(ctx context.Context, id string, stderr bool) (string, error) {
	return LogTail(ctx, a.Options, a.Clock, id, stderr)
}

// DaemonRunning uses the same lock-holder check as Doctor, without invoking service tools.
func (a TUI) DaemonRunning(context.Context) (bool, error) {
	cfg, err := config.Load(a.Options.configOptions())
	if err != nil {
		return false, err
	}
	return daemon.Holder(cfg.DataDir) != 0, nil
}

func (a TUI) Approve(ctx context.Context, name string) error {
	return ApproveJob(ctx, a.Options, a.Clock, name)
}
