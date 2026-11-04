package daemon

import (
	"context"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
	"testing"
)

type executeFunc func(context.Context, jobs.Job, store.Run, bool) (store.Run, error)

func (f executeFunc) Run(ctx context.Context, j jobs.Job, r store.Run, approved bool) (store.Run, error) {
	return f(ctx, j, r, approved)
}
func TestShutdownPreservesFinishedOutcomes(t *testing.T) {
	for _, state := range []string{"succeeded", "failed", "attention", "timed_out", "cancelled"} {
		t.Run(state, func(t *testing.T) {
			e, base := fixture(t)
			ctx, cancel := context.WithCancel(base)
			defer cancel()
			r, err := store.NewRun("job", "manual", e.Clock.Now())
			check(t, err)
			check(t, e.DB.CreateRun(base, r))
			check(t, e.DB.ClaimRun(base, r.ID))
			e.Runner = executeFunc(func(_ context.Context, _ jobs.Job, r store.Run, _ bool) (store.Run, error) {
				at := e.Clock.Now()
				pid := 123
				check(t, e.DB.StartRun(base, r.ID, pid, at, ""))
				r.State = state
				r.EndedAt = &at
				check(t, e.DB.FinishRun(base, r))
				cancel()
				return r, nil
			})
			e.Start(ctx, []store.Run{r}, map[string]jobs.Job{"job": jobs.Defaults("job")})
			e.Wait()
			got, err := e.DB.GetRun(base, r.ID)
			check(t, err)
			if got.State != state {
				t.Fatalf("want %s, got %+v", state, got)
			}
			if state == "cancelled" && got.Error != "daemon stopped" {
				t.Fatal(got)
			}
		})
	}
}
func TestShutdownSQLOnlyUpdatesCancelled(t *testing.T) {
	e, ctx := fixture(t)
	r, err := store.NewRun("job", "manual", e.Clock.Now())
	check(t, err)
	check(t, e.DB.CreateRun(ctx, r))
	check(t, e.DB.ClaimRun(ctx, r.ID))
	at := e.Clock.Now()
	check(t, e.DB.StartRun(ctx, r.ID, 123, at, ""))
	r.State = "succeeded"
	r.EndedAt = &at
	check(t, e.DB.FinishRun(ctx, r))
	check(t, e.DB.SetDaemonStopped(ctx, r.ID))
	got, err := e.DB.GetRun(ctx, r.ID)
	check(t, err)
	if got.State != "succeeded" {
		t.Fatal(got)
	}
}
