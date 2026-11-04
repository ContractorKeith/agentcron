package daemon

import (
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
	"testing"
	"time"
)

func TestBothPoliciesKeepRecentFiresScheduled(t *testing.T) {
	for _, policy := range []string{"one", "none"} {
		for _, age := range []time.Duration{time.Second, 29 * time.Second, 31 * time.Second} {
			t.Run(policy+age.String(), func(t *testing.T) {
				e, ctx := fixture(t)
				fire := e.Clock.Now().Add(-age)
				j := jobs.Defaults("job")
				j.Schedule = "at " + fire.Format(time.RFC3339)
				j.Catchup = policy
				st := store.JobState{Name: j.Name, Approved: true, NextFire: &fire, UpdatedAt: e.Clock.Now()}
				check(t, e.DB.PutJobState(ctx, st))
				check(t, e.Enqueue(ctx, map[string]jobs.Job{j.Name: j}, map[string]store.JobState{j.Name: st}))
				rows, err := e.DB.ListRuns(ctx, j.Name, "", -1)
				check(t, err)
				if age > 30*time.Second && policy == "none" {
					if len(rows) != 0 {
						t.Fatal(rows)
					}
					return
				}
				want := "schedule"
				if age > 30*time.Second {
					want = "catchup"
				}
				if len(rows) != 1 || rows[0].Trigger != want {
					t.Fatalf("want %s: %+v", want, rows)
				}
			})
		}
	}
}
