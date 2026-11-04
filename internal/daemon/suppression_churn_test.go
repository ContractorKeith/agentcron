package daemon

import (
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
	"testing"
	"time"
)

func TestSuppressedCursorWritesOnlyWhenChanged(t *testing.T) {
	for _, kind := range []string{"paused", "unapproved", "orphaned"} {
		t.Run(kind, func(t *testing.T) {
			e, ctx := fixture(t)
			now := e.Clock.Now()
			fire := now.Add(time.Minute)
			j := jobs.Defaults("job")
			j.Schedule = "every 1m"
			st := store.JobState{Name: j.Name, Approved: kind != "unapproved", Paused: kind == "paused", Orphaned: kind == "orphaned", AnchorAt: now, NextFire: &fire, UpdatedAt: now}
			check(t, e.DB.PutJobState(ctx, st))
			defs := map[string]jobs.Job{j.Name: j}
			// Make even an UPDATE writing identical values fail: no cursor write is allowed.
			check(t, e.DB.WithImmediateTx(ctx, func(tx *store.ImmediateTx) error {
				_, err := tx.Exec(ctx, `CREATE TRIGGER reject_churn BEFORE UPDATE ON job_state BEGIN SELECT RAISE(FAIL,'cursor churn'); END`)
				return err
			}))
			e.Clock.(*clock.Fake).Advance(10 * time.Second)
			check(t, e.Enqueue(ctx, defs, map[string]store.JobState{j.Name: st}))
			check(t, e.DB.WithImmediateTx(ctx, func(tx *store.ImmediateTx) error { _, err := tx.Exec(ctx, `DROP TRIGGER reject_churn`); return err }))
			e.Clock.(*clock.Fake).Advance(time.Minute)
			check(t, e.Enqueue(ctx, defs, map[string]store.JobState{j.Name: st}))
			saved, err := e.DB.JobStates(ctx)
			check(t, err)
			if saved[j.Name].NextFire == nil || !saved[j.Name].NextFire.After(e.Clock.Now()) || !saved[j.Name].UpdatedAt.Equal(e.Clock.Now()) {
				t.Fatal(saved)
			}
		})
	}
}
