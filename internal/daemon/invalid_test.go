package daemon

import (
	"context"
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
	"testing"
	"time"
)

type observedClock struct {
	*clock.Fake
	delays chan time.Duration
}

func (c observedClock) After(d time.Duration) <-chan time.Time { c.delays <- d; return c.Fake.After(d) }
func TestLoopFloorsStaleFireDelay(t *testing.T) {
	e, base := fixture(t)
	ctx, cancel := context.WithCancel(base)
	defer cancel()
	c := observedClock{e.Clock.(*clock.Fake), make(chan time.Duration, 1)}
	e.Clock = c
	past := c.Now().Add(-time.Hour)
	check(t, e.DB.PutJobState(ctx, store.JobState{Name: "invalid", Approved: true, NextFire: &past, UpdatedAt: c.Now()}))
	e.Sync = func(context.Context) (map[string]jobs.Job, error) { return map[string]jobs.Job{}, nil }
	done := make(chan error, 1)
	go func() { done <- e.Loop(ctx, nil) }()
	delay := <-c.delays
	cancel()
	check(t, <-done)
	if delay < time.Second {
		t.Fatalf("loop delay=%s, want >=1s", delay)
	}
}
