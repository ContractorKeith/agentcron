package daemon

import (
	"context"
	"testing"

	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func TestShutdownAppendsReasonToMemoryDiagnostic(t *testing.T) {
	e, base := fixture(t)
	ctx, cancel := context.WithCancel(base)
	defer cancel()
	r, err := store.NewRun("job", "manual", e.Clock.Now())
	check(t, err)
	check(t, e.DB.CreateRun(base, r))
	check(t, e.DB.ClaimRun(base, r.ID))
	const diagnostic = "memory unavailable: mdkb not available"
	e.Runner = executeFunc(func(_ context.Context, _ jobs.Job, r store.Run, _ bool) (store.Run, error) {
		at := e.Clock.Now()
		check(t, e.DB.StartRun(base, r.ID, 123, at, ""))
		r.State = "cancelled"
		r.Error = diagnostic
		r.EndedAt = &at
		check(t, e.DB.FinishRun(base, r))
		cancel()
		return r, nil
	})
	e.Start(ctx, []store.Run{r}, map[string]jobs.Job{"job": jobs.Defaults("job")})
	e.Wait()
	got, err := e.DB.GetRun(base, r.ID)
	check(t, err)
	if got.State != "cancelled" || got.Error != diagnostic+"; daemon stopped" {
		t.Fatalf("%+v", got)
	}
}
