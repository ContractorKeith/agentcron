package daemon

import (
	"github.com/ContractorKeith/agentcron/internal/store"
	"testing"
	"time"
)

func TestReconcileStopsGroupAfterLeaderReaped(t *testing.T) {
	e, ctx := fixture(t)
	at := e.Clock.Now()
	pid := 123
	r, err := store.NewRun("job", "manual", at)
	check(t, err)
	check(t, e.DB.CreateRun(ctx, r))
	check(t, e.DB.ClaimRun(ctx, r.ID))
	r.PID = &pid
	r.ProcStart = &at
	r.StartedAt = &at
	check(t, e.DB.StartOwnedRun(ctx, r))
	killed := false
	p := Processes{Alive: func(int) bool { return false }, Start: func(int) (time.Time, error) { return at, nil }, GroupExists: func(int) bool { return true }, Kill: func(got int) error {
		if got != pid {
			t.Errorf("pgid=%d", got)
		}
		current, err := e.DB.GetRun(ctx, r.ID)
		check(t, err)
		if current.State != "running" {
			t.Error("recorded unknown before group teardown")
		}
		killed = true
		return nil
	}}
	check(t, e.reconcile(ctx, p))
	got, err := e.DB.GetRun(ctx, r.ID)
	check(t, err)
	if !killed || got.State != "unknown" {
		t.Fatalf("group killed=%t run=%+v", killed, got)
	}
}
