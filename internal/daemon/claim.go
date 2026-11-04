package daemon

import (
	"context"

	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func (e *Engine) Claim(ctx context.Context, defs map[string]jobs.Job, states map[string]store.JobState) ([]store.Run, error) {
	all, err := e.DB.ListRuns(ctx, "", "", -1)
	if err != nil {
		return nil, err
	}
	busy := map[string]bool{}
	count := 0
	for _, r := range all {
		if r.State == "claimed" || r.State == "running" {
			busy[r.Job] = true
			count++
		}
	}
	result := []store.Run{}
	for i := len(all) - 1; i >= 0; i-- {
		r := all[i]
		if r.State != "queued" {
			continue
		}
		st := states[r.Job]
		_, exists := defs[r.Job]
		if busy[r.Job] || !exists || st.Paused || !st.Approved || st.Orphaned {
			at := e.Clock.Now()
			r.State = "skipped"
			r.EndedAt = &at
			r.Error = "job unavailable or already running"
			if err := e.DB.FinishRun(ctx, r); err != nil {
				return nil, err
			}
			e.log(r)
			continue
		}
		if count >= e.Concurrency {
			continue
		}
		if err := e.DB.ClaimOwned(ctx, r.ID, e.Clock.Now()); err != nil {
			return nil, err
		}
		r.State = "claimed"
		busy[r.Job] = true
		count++
		result = append(result, r)
		e.log(r)
	}
	return result, nil
}
