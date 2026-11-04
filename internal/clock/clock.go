// Package clock supplies wall time and deterministic timers.
package clock

import (
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
	After(time.Duration) <-chan time.Time
}

type Real struct{}

func (Real) Now() time.Time                         { return time.Now() }
func (Real) After(d time.Duration) <-chan time.Time { return time.After(d) }

type timer struct {
	at time.Time
	ch chan time.Time
}

type Fake struct {
	mu      sync.Mutex
	now     time.Time
	timers  []timer
	changed *sync.Cond
}

func NewFake(start time.Time) *Fake {
	f := &Fake{now: start}
	f.changed = sync.NewCond(&f.mu)
	return f
}
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}
func (f *Fake) After(d time.Duration) <-chan time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch := make(chan time.Time, 1)
	at := f.now.Add(d)
	if d <= 0 {
		ch <- f.now
	} else {
		f.timers = append(f.timers, timer{at, ch})
		f.changed.Broadcast()
	}
	return ch
}

// Advance delivers due timers without blocking on their consumers.
func (f *Fake) Advance(d time.Duration) {
	if d < 0 {
		panic("clock: cannot advance backwards")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
	pending := f.timers[:0]
	for _, t := range f.timers {
		if !t.at.After(f.now) {
			t.ch <- t.at
		} else {
			pending = append(pending, t)
		}
	}
	f.timers = pending
}

// BlockUntil waits until n pending timers have been registered.
func (f *Fake) BlockUntil(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for len(f.timers) < n {
		f.changed.Wait()
	}
}
