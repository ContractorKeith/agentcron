package daemon

import (
	"github.com/ContractorKeith/agentcron/internal/store"
	"testing"
	"time"
)

func TestReconcileOwnerAndConcurrentFinish(t *testing.T) {
	for _, liveOwner := range []bool{true, false} {
		t.Run(map[bool]string{true: "live-owner-dead-leader", false: "concurrent-finish"}[liveOwner], func(t *testing.T) {
			e, ctx := fixture(t)
			at := e.Clock.Now()
			pid, owner := 123, 456
			r, err := store.NewRun("job", "manual", at)
			check(t, err)
			check(t, e.DB.CreateRun(ctx, r))
			check(t, e.DB.ClaimRun(ctx, r.ID))
			r.PID = &pid
			r.OwnerPID = &owner
			r.OwnerStart = &at
			r.ProcStart = &at
			r.StartedAt = &at
			check(t, e.DB.StartOwnedRun(ctx, r))
			kills := 0
			p := Processes{Alive: func(p int) bool { return p == owner && liveOwner }, Start: func(int) (time.Time, error) { return at, nil }, GroupExists: func(int) bool { return true }, Kill: func(int) error {
				kills++
				r.State = "succeeded"
				r.EndedAt = &at
				return e.DB.FinishRun(ctx, r)
			}}
			check(t, e.reconcile(ctx, p))
			got, err := e.DB.GetRun(ctx, r.ID)
			check(t, err)
			if liveOwner {
				if got.State != "running" || kills != 0 {
					t.Fatalf("state=%s kills=%d", got.State, kills)
				}
			} else if got.State != "succeeded" || kills != 1 {
				t.Fatalf("state=%s kills=%d", got.State, kills)
			}
		})
	}
}
