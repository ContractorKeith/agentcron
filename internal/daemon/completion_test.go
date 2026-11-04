package daemon

import (
	"context"
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
	"testing"
	"time"
)

func TestCompletionImmediatelyClaimsQueuedJobs(t *testing.T) {
	e, base := fixture(t)
	e.Concurrency = 1
	c := e.Clock.(*clock.Fake)
	ctx, cancel := context.WithCancel(base)
	defer cancel()
	defs := map[string]jobs.Job{}
	for _, name := range []string{"a", "b", "c"} {
		defs[name] = jobs.Defaults(name)
		check(t, e.DB.PutJobState(ctx, store.JobState{Name: name, Approved: true, UpdatedAt: c.Now()}))
		r, err := store.NewRun(name, "manual", c.Now())
		check(t, err)
		check(t, e.DB.CreateRun(ctx, r))
	}
	e.Sync = func(context.Context) (map[string]jobs.Job, error) { return defs, nil }
	first := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan string, 3)
	e.Runner = executeFunc(func(ctx context.Context, j jobs.Job, r store.Run, _ bool) (store.Run, error) {
		at := c.Now()
		if err := e.DB.StartRun(ctx, r.ID, 123, at, ""); err != nil {
			return r, err
		}
		if j.Name == "a" {
			close(first)
			select {
			case <-release:
			case <-ctx.Done():
			}
		}
		r.State = "succeeded"
		r.EndedAt = &at
		err := e.DB.FinishRun(context.WithoutCancel(ctx), r)
		finished <- j.Name
		return r, err
	})
	done := make(chan error, 1)
	go func() { done <- e.Loop(ctx, nil) }()
	<-first
	c.BlockUntil(1)
	close(release)
	// This is only a deadlock guard: the scheduler's fake clock never advances.
	guard := (clock.Real{}).After(5 * time.Second)
	for _, want := range []string{"a", "b", "c"} {
		select {
		case got := <-finished:
			if got != want {
				t.Errorf("got %s want %s", got, want)
			}
		case <-guard:
			cancel()
			<-done
			t.Fatal("queue did not wake on completion")
		}
	}
	cancel()
	check(t, <-done)
}
