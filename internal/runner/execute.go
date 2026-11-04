package runner

import (
	"context"
	"errors"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

// Run records every pre-launch failure, including failures before preflight.
func (x Runner) Run(ctx context.Context, j jobs.Job, r store.Run, approved bool) (store.Run, error) {
	result, err := x.run(ctx, j, r, approved)
	if err == nil {
		return result, nil
	}
	persist := context.WithoutCancel(ctx)
	current, readErr := x.DB.GetRun(persist, r.ID)
	if readErr != nil {
		return result, errors.Join(err, readErr)
	}
	if current.State == "queued" {
		if claimErr := x.DB.ClaimRun(persist, r.ID); claimErr != nil {
			return result, errors.Join(err, claimErr)
		}
		current.State = "claimed"
	}
	if current.State != "claimed" {
		return result, err
	}
	current.State = "blocked_config"
	current.Error = err.Error()
	at := x.Clock.Now()
	current.EndedAt = &at
	if finishErr := x.DB.FinishRun(persist, current); finishErr != nil {
		return result, errors.Join(err, finishErr)
	}
	return current, nil
}
