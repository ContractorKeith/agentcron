package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

var errSetupTimeout = errors.New("setup timed out")

func (x Runner) setup(ctx context.Context, j jobs.Job, r store.Run, approved bool) (prepared, store.Run, error) {
	timeout, _ := time.ParseDuration(j.Timeout)
	budget, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	setupCtx, stop := context.WithCancelCause(budget)
	defer stop(nil)
	p, err := preflight(setupCtx, j, approved, x.JobsDir)
	if err == nil {
		r.Worktree, err = createWorktree(setupCtx, x.DataDir, j, r, p)
	}
	dir := filepath.Join(x.DataDir, "runs", r.ID)
	if err == nil {
		err = os.MkdirAll(dir, 0700)
	}
	if err == nil && j.Memory {
		done := x.watchSetupCancel(setupCtx, dir, stop)
		defer func() { stop(nil); <-done }()
		var memoryErr error
		p.memory, memoryErr = x.prepareMemory(setupCtx, j, dir)
		if memoryErr != nil {
			r.Error = memoryErr.Error()
		}
	}
	if err == nil && setupCancelled(dir) {
		err = errSetupCancelled
	}
	if budget.Err() == context.DeadlineExceeded {
		err = errors.Join(errSetupTimeout, err)
	} else if setupCtx.Err() != nil {
		err = errors.Join(context.Cause(setupCtx), err)
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
