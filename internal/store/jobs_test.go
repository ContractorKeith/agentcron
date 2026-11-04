package store

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestJobStateRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, scratchDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	anchor := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	next := anchor.Add(time.Hour)
	want := JobState{Name: "morning", Paused: true, Approved: true, NextFire: &next, LastRunID: "r_one", UpdatedAt: anchor, AnchorAt: anchor}
	if err := s.PutJobState(ctx, want); err != nil {
		t.Fatal(err)
	}
	states, err := s.JobStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := states["morning"]
	if got.Name != want.Name || !got.Paused || !got.Approved || !got.NextFire.Equal(next) || got.LastRunID != want.LastRunID || !got.AnchorAt.Equal(anchor) {
		t.Fatal(got)
	}
	want.Orphaned = true
	want.NextFire = nil
	want.Paused = false
	if err := s.PutJobState(ctx, want); err != nil {
		t.Fatal(err)
	}
	states, err = s.JobStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := states["morning"]; !got.Orphaned || got.NextFire != nil || got.Paused {
		t.Fatal(got)
	}
}
func TestUpgradeMigrationOne(t *testing.T) {
	ctx := context.Background()
	path := scratchDB(t)
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	// Rebuild a genuine v1 database, including an existing state row.
	if _, err = s.db.Exec("DROP TABLE job_state; DELETE FROM schema_version WHERE version=2"); err != nil {
		t.Fatal(err)
	}
	initial, err := os.ReadFile("migrations/001_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	// Only the job-state declaration is needed; the ledger already exists.
	end := 0
	for i, b := range initial {
		if b == ';' {
			end = i + 1
			break
		}
	}
	if _, err = s.db.Exec(string(initial[:end])); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("INSERT INTO job_state(name,paused,approved,updated_at) VALUES('old',1,1,'2026-11-04T00:00:00Z')"); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	states, err := s.JobStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := states["old"]; !got.Paused || !got.Approved || got.Orphaned || !got.AnchorAt.IsZero() {
		t.Fatal(got)
	}
}
