package store

import (
	"context"
	"testing"
	"time"
)

func TestJobStateUTC(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, scratchDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	local := time.Date(2026, 11, 4, 7, 0, 0, 0, time.FixedZone("EST", -5*60*60))
	st := JobState{Name: "utc", UpdatedAt: local, AnchorAt: local, NextFire: &local}
	if err := s.PutJobState(ctx, st); err != nil {
		t.Fatal(err)
	}
	var updated, anchor, next string
	if err := s.db.QueryRow("SELECT updated_at,anchor_at,next_fire FROM job_state WHERE name='utc'").Scan(&updated, &anchor, &next); err != nil {
		t.Fatal(err)
	}
	for _, got := range []string{updated, anchor, next} {
		if got != "2026-11-04T12:00:00Z" {
			t.Errorf("stored timestamp=%q", got)
		}
	}
	// Previously persisted timestamps with offsets are normalized on read too.
	if _, err := s.db.Exec("UPDATE job_state SET updated_at=?,anchor_at=?,next_fire=? WHERE name='utc'", local.Format(time.RFC3339), local.Format(time.RFC3339), local.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	states, err := s.JobStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := states["utc"]
	for _, at := range []time.Time{got.UpdatedAt, got.AnchorAt, *got.NextFire} {
		if at.Format(time.RFC3339) != "2026-11-04T12:00:00Z" {
			t.Errorf("read timestamp=%s", at.Format(time.RFC3339))
		}
	}
}
