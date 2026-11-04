package store

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCacheSurvivesStateUpdatesAndReopen(t *testing.T) {
	ctx := context.Background()
	path := scratchDB(t)
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.MemoryNote(ctx, "job"); err != nil || got != "" {
		t.Fatalf("%q %v", got, err)
	}
	state := JobState{Name: "job", Approved: true, Paused: true, UpdatedAt: time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)}
	if err = s.PutJobState(ctx, state); err != nil {
		t.Fatal(err)
	}
	if err = s.SetMemoryNote(ctx, "job", "cache-key"); err != nil {
		t.Fatal(err)
	}
	state.Paused = false
	if err = s.PutJobState(ctx, state); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenExisting(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got, err := s.MemoryNote(ctx, "job"); err != nil || got != "cache-key" {
		t.Fatalf("%q %v", got, err)
	}
	states, err := s.JobStates(ctx)
	if err != nil || !states["job"].Approved || states["job"].Paused {
		t.Fatalf("%+v %v", states, err)
	}
}

func TestMemoryVerificationNeverCreatesJobState(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, scratchDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.SetMemoryNote(ctx, "absent", "verified"); err != nil {
		t.Fatal(err)
	}
	states, err := s.JobStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := states["absent"]; ok {
		t.Fatal("memory verification created job state")
	}
}
