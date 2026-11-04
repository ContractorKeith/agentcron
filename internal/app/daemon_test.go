package app

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func TestSyncPreservesMissedFire(t *testing.T) {
	opts := isolated(t)
	ctx := context.Background()
	c := clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 987, time.UTC))
	if err := Init(ctx, opts); err != nil {
		t.Fatal(err)
	}
	j := NewJob("interval")
	j.Schedule = "every 1m"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hello"
	if _, err := AddJob(ctx, opts, c, j, false); err != nil {
		t.Fatal(err)
	}
	before, err := ShowJob(ctx, opts, c, j.Name)
	if err != nil {
		t.Fatal(err)
	}
	c.Advance(10 * time.Minute)
	after, err := ShowJob(ctx, opts, c, j.Name)
	if err != nil {
		t.Fatal(err)
	}
	if after.State.NextFire == nil || !after.State.NextFire.Equal(*before.State.NextFire) || after.State.AnchorAt.Nanosecond() != 0 {
		t.Fatal(after)
	}
	j.Schedule = "every 2m"
	if err := jobs.Write(filepath.Join(opts.ConfigDir, "jobs"), j); err != nil {
		t.Fatal(err)
	}
	after, err = ShowJob(ctx, opts, c, j.Name)
	if err != nil {
		t.Fatal(err)
	}
	if !after.State.NextFire.After(c.Now()) {
		t.Fatal("changed definition retained old cursor", after)
	}
}
func TestJobStatusWords(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	j := NewJob("job")
	j.Schedule = "at " + now.Format(time.RFC3339)
	for _, tc := range []struct {
		state store.JobState
		runs  []store.Run
		want  string
	}{
		{store.JobState{Orphaned: true}, nil, "orphaned"},
		{store.JobState{}, nil, "unapproved"},
		{store.JobState{Approved: true, Paused: true}, nil, "paused"},
		{store.JobState{Approved: true, NextFire: &now}, nil, "active"},
		{store.JobState{Approved: true}, nil, "never"},
		{store.JobState{Approved: true}, []store.Run{{Trigger: "schedule"}}, "done"},
	} {
		if got := jobStatus(j, tc.state, tc.runs); got != tc.want {
			t.Fatal(got, tc.want)
		}
	}
}

func TestOneShotDoneAfterRetention(t *testing.T) {
	opts := isolated(t)
	ctx := context.Background()
	c := clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
	if err := Init(ctx, opts); err != nil {
		t.Fatal(err)
	}
	j := NewJob("once")
	j.Schedule = "at " + c.Now().Add(time.Minute).Format(time.RFC3339)
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hello"
	if _, err := AddJob(ctx, opts, c, j, false); err != nil {
		t.Fatal(err)
	}
	// No terminal ledger history remains, but the persisted scheduling cursor remembers the fire.
	db, err := store.Open(ctx, filepath.Join(opts.DataDir, "agentcron.db"))
	if err != nil {
		t.Fatal(err)
	}
	states, err := db.JobStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	st := states[j.Name]
	st.NextFire = nil
	st.Fired = true
	if err = db.AdvanceCursor(ctx, st); err != nil {
		t.Fatal(err)
	}
	db.Close()
	detail, err := ShowJob(ctx, opts, c, j.Name)
	if err != nil || detail.Status != "done" || len(detail.Runs) != 0 {
		t.Fatal(detail, err)
	}
}

func TestRestartRemovesClaimedPartialWorktree(t *testing.T) {
	opts := isolated(t)
	ctx := context.Background()
	c := clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
	if err := Init(ctx, opts); err != nil {
		t.Fatal(err)
	}
	j := NewJob("partial")
	j.Schedule = "every 1h"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hello"
	if _, err := AddJob(ctx, opts, c, j, false); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(ctx, filepath.Join(opts.DataDir, "agentcron.db"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := store.NewRun(j.Name, "manual", c.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = db.CreateRun(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err = db.ClaimRun(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	db.Close()
	partial := filepath.Join(opts.DataDir, "worktrees", j.Name, r.ID)
	if err = os.MkdirAll(partial, 0700); err != nil {
		t.Fatal(err)
	}
	if err = Schedule(ctx, opts, c, false, false, nil, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(partial); !os.IsNotExist(err) {
		t.Fatal("partial worktree survived", err)
	}
	db, err = store.Open(ctx, filepath.Join(opts.DataDir, "agentcron.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	got, err := db.GetRun(ctx, r.ID)
	if err != nil || got.State != "unknown" || got.Kept {
		t.Fatal(got, err)
	}
}
