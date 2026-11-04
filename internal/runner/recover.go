package runner

import (
	"context"
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
	"time"
)

// RecoverClaimed removes a partial setup left by a dead owner.
func RecoverClaimed(ctx context.Context, j jobs.Job, r *store.Run) error {
	kept, err := rollbackWorktree(ctx, j, *r)
	r.Kept = kept
	return err
}

// TeardownGroup drains descendants even after their group leader has been reaped.
func TeardownGroup(pid int, c clock.Clock, grace time.Duration) { teardownGroup(pid, c, grace) }
