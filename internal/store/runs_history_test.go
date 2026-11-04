package store

import (
	"context"
	"testing"
	"time"
)

func TestPreviousRunSkipsNonterminal(t *testing.T) {
	ctx := context.Background()
	s, e := Open(ctx, scratchDB(t))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	at := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	var first, last Run
	for i := range 3 {
		r, e := NewRun("job", "manual", at)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.CreateRun(ctx, r); e != nil {
			t.Fatal(e)
		}
		if i == 0 {
			r.State = "skipped"
			r.EndedAt = &at
			if e = s.FinishRun(ctx, r); e != nil {
				t.Fatal(e)
			}
			first = r
		}
		last = r
	}
	previous, e := s.PreviousRun(ctx, last)
	if e != nil || previous == nil || previous.ID != first.ID {
		t.Fatalf("%+v %v", previous, e)
	}
}

func TestPreviousRunIncludesCompletedLaterQueueEntry(t *testing.T) {
	ctx := context.Background()
	s, e := Open(ctx, scratchDB(t))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	at := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	queued, e := NewRun("job", "manual", at)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.CreateRun(ctx, queued); e != nil {
		t.Fatal(e)
	}
	later, e := NewRun("job", "manual", at)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.CreateRun(ctx, later); e != nil {
		t.Fatal(e)
	}
	later.State = "skipped"
	later.EndedAt = &at
	if e = s.FinishRun(ctx, later); e != nil {
		t.Fatal(e)
	}
	previous, e := s.PreviousRun(ctx, queued)
	if e != nil || previous == nil || previous.ID != later.ID {
		t.Fatalf("%+v %v", previous, e)
	}
}
