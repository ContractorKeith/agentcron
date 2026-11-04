package runner

import (
	"context"
	"errors"
	"time"

	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

var errSetupTimeout = errors.New("setup timed out")

func (x Runner) setup(ctx context.Context, j jobs.Job, r store.Run, approved bool) (prepared, store.Run, error) {
	timeout, _ := time.ParseDuration(j.Timeout)
	setupCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	p, err := preflight(setupCtx, j, approved, x.JobsDir)
	if err == nil {
		r.Worktree, err = createWorktree(setupCtx, x.DataDir, j, r, p)
	}
	if setupCtx.Err() == context.DeadlineExceeded {
		err = errors.Join(errSetupTimeout, err)
	}
	if err != nil && r.Worktree != "" {
		cleanupCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		var cleanupErr error
		r.Kept, cleanupErr = rollbackWorktree(cleanupCtx, j, r)
		err = errors.Join(err, cleanupErr)
	}
	return p, r, err
}
