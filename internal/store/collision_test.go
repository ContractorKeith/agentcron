package store

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunIDHasEightRandomCharacters(t *testing.T) {
	r, err := NewRun("job", "manual", time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	suffix := r.ID[strings.LastIndexByte(r.ID, '_')+1:]
	if len(suffix) != 8 {
		t.Fatalf("random suffix=%q", suffix)
	}
}
func TestScheduledIDCollisionDoesNotDropFire(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, scratchDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	at := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	first, err := NewRun("first", "schedule", at)
	if err != nil {
		t.Fatal(err)
	}
	if _, inserted, err := s.EnqueueFire(ctx, first); err != nil || !inserted {
		t.Fatal(err)
	}
	second := first
	second.Job = "second"
	if _, inserted, err := s.EnqueueFire(ctx, second); err != nil || !inserted {
		t.Fatal(err)
	}
	rows, err := s.ListRuns(ctx, second.Job, "", -1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("fire dropped: %+v error=%v", rows, err)
	}
	if rows[0].ID == first.ID {
		t.Fatal("did not regenerate collided ID")
	}
}

func TestScheduledFireConflictReportsNotInserted(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, scratchDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := NewRun("job", "schedule", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if _, inserted, err := s.EnqueueFire(ctx, r); err != nil || !inserted {
		t.Fatal(inserted, err)
	}
	r.ID = "different"
	r.Trigger = "catchup"
	if _, inserted, err := s.EnqueueFire(ctx, r); err != nil || inserted {
		t.Fatal(inserted, err)
	}
}
