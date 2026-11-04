package tui

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/ContractorKeith/agentcron/internal/clock"
)

type observedClock struct {
	*clock.Fake
	registered chan time.Duration
}

func (c observedClock) After(d time.Duration) <-chan time.Time {
	ch := c.Fake.After(d)
	c.registered <- d
	return ch
}

func TestFakeClockFiresOneRefresh(t *testing.T) {
	m, f := fixture()
	c := observedClock{Fake: m.clock.(*clock.Fake), registered: make(chan time.Duration, 1)}
	m.clock = c
	ctx, cancel := context.WithCancel(m.ctx)
	m.ctx = ctx
	result := make(chan tea.Msg, 1)
	batch := m.Init()().(tea.BatchMsg)
	done := make(chan struct{})
	go func() { defer close(done); result <- batch[1]() }()
	defer func() { cancel(); <-done }()
	select {
	case d := <-c.registered:
		if d != time.Second {
			t.Fatalf("tick interval %s", d)
		}
	case <-time.After(time.Second):
		t.Fatal("tick did not use injected clock")
	}
	c.Advance(999 * time.Millisecond)
	select {
	case <-result:
		t.Fatal("refresh fired before one second")
	default:
	}
	before := len(f.calls)
	c.Advance(time.Millisecond)
	select {
	case msg := <-result:
		next, cmd := m.Update(msg)
		refresh := cmd().(tea.BatchMsg)[0]
		m = deliver(next.(model), refresh)
		if len(f.calls) != before+1 || f.calls[len(f.calls)-1] != "runs daily" {
			t.Fatalf("expected one refresh: %v", f.calls)
		}
	case <-time.After(time.Second):
		t.Fatal("fake clock did not fire refresh")
	}
}
