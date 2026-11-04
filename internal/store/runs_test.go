package store

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunTransitions(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, scratchDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	at := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	for _, state := range []string{"succeeded", "failed", "timed_out", "cancelled", "attention", "unknown", "skipped", "blocked_config"} {
		t.Run(state, func(t *testing.T) {
			r, e := NewRun("job", "manual", at)
			if e != nil {
				t.Fatal(e)
			}
			if e = s.CreateRun(ctx, r); e != nil {
				t.Fatal(e)
			}
			got, e := s.GetRun(ctx, r.ID)
			if e != nil || got.State != "queued" {
				t.Fatalf("%+v %v", got, e)
			}
			if state != "skipped" {
				if e = s.ClaimRun(ctx, r.ID); e != nil {
					t.Fatal(e)
				}
			}
			if state != "skipped" && state != "blocked_config" {
				if e = s.StartRun(ctx, r.ID, 123, at, "/temp"); e != nil {
					t.Fatal(e)
				}
			}
			r.State = state
			r.EndedAt = &at
			r.Error = "details"
			if e = s.FinishRun(ctx, r); e != nil {
				t.Fatal(e)
			}
			got, e = s.GetRun(ctx, r.ID)
			if e != nil || got.State != state || got.Error != "details" {
				t.Fatalf("%+v %v", got, e)
			}
			if e = s.StartRun(ctx, r.ID, 123, at, ""); e == nil {
				t.Fatal("terminal run restarted")
			}
			if e = s.ClaimRun(ctx, r.ID); e == nil {
				t.Fatal("terminal run claimed")
			}
		})
	}
	rows, e := s.ListRuns(ctx, "job", "failed", 1)
	if e != nil || len(rows) != 1 || rows[0].State != "failed" {
		t.Fatalf("%+v %v", rows, e)
	}
}
func TestConcurrentRunClaim(t *testing.T) {
	ctx := context.Background()
	path := scratchDB(t)
	s, e := Open(ctx, path)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	other, e := Open(ctx, path)
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	r, e := NewRun("job", "manual", time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC))
	if e != nil {
		t.Fatal(e)
	}
	if e = s.CreateRun(ctx, r); e != nil {
		t.Fatal(e)
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db := s
			if i%2 == 0 {
				db = other
			}
			if db.ClaimRun(ctx, r.ID) == nil {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("winners=%d", wins.Load())
	}
}

func TestPreviousRunExcludesLaterQueueEntries(t *testing.T) {
	ctx := context.Background()
	s, e := Open(ctx, scratchDB(t))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	at := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	runs := []Run{}
	for range 3 {
		r, e := NewRun("job", "manual", at)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.CreateRun(ctx, r); e != nil {
			t.Fatal(e)
		}
		if len(runs) == 0 {
			r.State = "skipped"
			r.EndedAt = &at
			if e = s.FinishRun(ctx, r); e != nil {
				t.Fatal(e)
			}
		}
		runs = append(runs, r)
	}
	previous, e := s.PreviousRun(ctx, runs[1])
	if e != nil || previous == nil || previous.ID != runs[0].ID {
		t.Fatalf("%+v %v", previous, e)
	}
	previous, e = s.PreviousRun(ctx, runs[0])
	if e != nil || previous != nil {
		t.Fatalf("%+v %v", previous, e)
	}
}
