package runner

import (
	"context"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

// RecoverClaimed removes a partial setup left by a dead owner.
func RecoverClaimed(ctx context.Context, j jobs.Job, r *store.Run) error {
	kept, err := rollbackWorktree(ctx, j, *r)
	r.Kept = kept
	return err
}
