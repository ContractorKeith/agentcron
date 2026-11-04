package daemon

import (
	"github.com/ContractorKeith/agentcron/internal/store"
	"testing"
	"time"
)

func TestRecycledOwnerDoesNotStrandRun(t *testing.T) {
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
	killed := false
	p := Processes{Alive: func(int) bool { return true }, Start: func(pid int) (time.Time, error) {
		if pid == owner {
			return at.Add(time.Hour), nil
		}
		return at, nil
	}, Kill: func(int) error { killed = true; return nil }}
	check(t, e.reconcile(ctx, p))
	got, err := e.DB.GetRun(ctx, r.ID)
	check(t, err)
	if got.State != "unknown" || !killed {
		t.Fatalf("run=%+v killed=%t", got, killed)
	}
}
