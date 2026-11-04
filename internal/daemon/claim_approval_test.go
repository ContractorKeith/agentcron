package daemon

import (
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
	"testing"
)

func TestQueuedUnapprovedNeverClaimed(t *testing.T) {
	e, ctx := fixture(t)
	r, err := store.NewRun("job", "manual", e.Clock.Now())
	check(t, err)
	check(t, e.DB.CreateRun(ctx, r))
	claimed, err := e.Claim(ctx, map[string]jobs.Job{"job": {}}, map[string]store.JobState{"job": {Approved: false, Paused: false}})
	check(t, err)
	got, err := e.DB.GetRun(ctx, r.ID)
	check(t, err)
	if len(claimed) != 0 || got.State != "skipped" {
		t.Fatalf("claimed=%v run=%+v", claimed, got)
	}
}
