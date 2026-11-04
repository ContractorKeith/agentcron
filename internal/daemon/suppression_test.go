package daemon

import (
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
	"testing"
	"time"
)

func TestSuppressedFiresAdvance(t *testing.T) {
	for _, kind := range []string{"paused", "unapproved", "orphaned"} {
		t.Run(kind, func(t *testing.T) {
			e, ctx := fixture(t)
			now := e.Clock.Now()
			first := now.Add(-3 * time.Minute)
			j := jobs.Defaults("job")
			j.Schedule = "every 1m"
			st := store.JobState{Name: j.Name, Approved: kind != "unapproved", Paused: kind == "paused", Orphaned: kind == "orphaned", AnchorAt: first, NextFire: &first, UpdatedAt: now}
			check(t, e.DB.PutJobState(ctx, st))
			defs := map[string]jobs.Job{j.Name: j}
			check(t, e.Enqueue(ctx, defs, map[string]store.JobState{j.Name: st}))
			states, err := e.DB.JobStates(ctx)
			check(t, err)
			st = states[j.Name]
			if st.NextFire == nil || !st.NextFire.After(now) {
				t.Fatalf("cursor not advanced: %+v", st)
			}
			st.Approved = true
			st.Paused = false
			st.Orphaned = false
			check(t, e.DB.PutJobState(ctx, st))
			check(t, e.Enqueue(ctx, defs, map[string]store.JobState{j.Name: st}))
			runs, err := e.DB.ListRuns(ctx, j.Name, "", -1)
			check(t, err)
			if len(runs) != 0 {
				t.Fatal(runs)
			}
			e.Clock.(*clock.Fake).Advance(time.Minute)
			check(t, e.Enqueue(ctx, defs, map[string]store.JobState{j.Name: st}))
			runs, err = e.DB.ListRuns(ctx, j.Name, "", -1)
			check(t, err)
			if len(runs) != 1 || runs[0].Trigger != "schedule" {
				t.Fatal(runs)
			}
		})
	}
}
