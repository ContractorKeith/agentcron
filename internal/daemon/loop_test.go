package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func TestLoopWakeAndReload(t *testing.T) {
	e, base := fixture(t)
	c := e.Clock.(*clock.Fake)
	ctx, cancel := context.WithCancel(base)
	defer cancel()
	synced := make(chan struct{}, 10)
	passes := 0
	e.Sync = func(context.Context) (map[string]jobs.Job, error) {
		passes++
		if passes == 2 {
			if err := e.DB.PutJobState(ctx, store.JobState{Name: "job", UpdatedAt: c.Now()}); err != nil {
				return nil, err
			}
		}
		synced <- struct{}{}
		return map[string]jobs.Job{}, nil
	}
	next := c.Now().Add(5 * time.Second)
	check(t, e.DB.PutJobState(ctx, store.JobState{Name: "job", Approved: true, NextFire: &next, UpdatedAt: c.Now()}))
	hup := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() { done <- e.Loop(ctx, hup) }()
	<-synced
	c.BlockUntil(1)
	c.Advance(5 * time.Second)
	<-synced

	c.BlockUntil(1)
	hup <- struct{}{}
	<-synced
	c.BlockUntil(2)
	cancel()
	check(t, <-done)
}
