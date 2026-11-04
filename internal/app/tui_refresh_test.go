package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func TestTUIRefreshLeavesLedgerUnchanged(t *testing.T) {
	ctx := context.Background()
	opts := isolated(t)
	c := clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
	if err := Init(ctx, opts); err != nil {
		t.Fatal(err)
	}
	j := NewJob("daily")
	j.Schedule = "@daily"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hello"
	if _, err := AddJob(ctx, opts, c, j, false); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenExisting(ctx, filepath.Join(opts.DataDir, "agentcron.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	before, err := db.JobStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c.Advance(time.Minute)
	api := TUI{Options: opts, Clock: c}
	js, err := api.Jobs(ctx)
	if err != nil || len(js) != 1 {
		t.Fatalf("jobs %v: %v", js, err)
	}
	after, err := db.JobStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !before["daily"].UpdatedAt.Equal(after["daily"].UpdatedAt) {
		t.Fatal("refresh wrote job state to the ledger")
	}
	// Read-only settings belong to reads, never to later explicit key actions.
	if err := api.Pause(ctx, "daily", true); err != nil {
		t.Fatal(err)
	}
	after, err = db.JobStates(ctx)
	if err != nil || !after["daily"].Paused {
		t.Fatalf("explicit pause failed: %v", err)
	}
}

func TestTUIApprovesAgentJob(t *testing.T) {
	ctx := context.Background()
	opts := isolated(t)
	t.Setenv("AGENTCRON_RUN_ID", "")
	c := clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
	if err := Init(ctx, opts); err != nil {
		t.Fatal(err)
	}
	j := NewJob("proposed")
	j.Schedule = "@daily"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hello"
	if _, err := CreateMCPJob(ctx, opts, c, j); err != nil {
		t.Fatal(err)
	}
	api := TUI{Options: opts, Clock: c}
	approver, ok := any(api).(interface {
		Approve(context.Context, string) error
	})
	if !ok {
		t.Fatal("application TUI does not implement approval")
	}
	if err := approver.Approve(ctx, "proposed"); err != nil {
		t.Fatal(err)
	}
	js, err := api.Jobs(ctx)
	if err != nil || len(js) != 1 || js[0].Status != "active" || !js[0].State.Approved || js[0].State.Paused {
		t.Fatalf("approval did not enable job: %+v %v", js, err)
	}
}
