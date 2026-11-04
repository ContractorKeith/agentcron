package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/config"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func TestJobLifecycle(t *testing.T) {
	opts := isolated(t)
	ctx := context.Background()
	c := clock.NewFake(time.Date(2026, 11, 4, 12, 13, 0, 0, time.UTC))
	if _, err := ListJobs(ctx, opts, c); Code(err) != NotInitialized {
		t.Fatal(err)
	}
	if err := Init(ctx, opts); err != nil {
		t.Fatal(err)
	}
	j := NewJob("morning")
	j.Schedule = "every 2h"
	j.Runner = "claude"
	j.Prompt = "hi"
	j.CreatedBy = "mcp"
	if _, err := AddJob(ctx, opts, c, j, true); err != nil {
		t.Fatal(err)
	}
	list, err := ListJobs(ctx, opts, c)
	if err != nil || len(list) != 0 {
		t.Fatal(list, err)
	}
	if _, err := AddJob(ctx, opts, c, j, false); err != nil {
		t.Fatal(err)
	}
	if _, err := AddJob(ctx, opts, c, j, false); Code(err) != Usage {
		t.Fatal(err)
	}
	detail, err := ShowJob(ctx, opts, c, "morning")
	if err != nil || !detail.State.Approved || detail.Job.CreatedBy != "cli" || len(detail.Runs) != 0 {
		t.Fatal(detail, err)
	}
	anchor := detail.State.AnchorAt
	c.Advance(45 * time.Minute)
	if err := PauseJob(ctx, opts, c, "morning", true); err != nil {
		t.Fatal(err)
	}
	detail, err = ShowJob(ctx, opts, c, "morning")
	if err != nil || !detail.State.Paused || !detail.Next[0].Equal(anchor.Add(2*time.Hour)) {
		t.Fatal(detail, err)
	}
	next, err := NextFires(ctx, opts, c, "", "", 5)
	if err != nil || len(next) != 0 {
		t.Fatal(next, err)
	}
	if err := PauseJob(ctx, opts, c, "morning", false); err != nil {
		t.Fatal(err)
	}
	if err := RemoveJob(ctx, opts, c, "morning", true); err != nil {
		t.Fatal(err)
	}
	if _, err := ShowJob(ctx, opts, c, "morning"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveJob(ctx, opts, c, "morning", false); err != nil {
		t.Fatal(err)
	}
	if detail, err := ShowJob(ctx, opts, c, "morning"); err != nil || detail.Status != "orphaned" {
		t.Fatal(detail, err)
	}
	cfg, _ := config.Load(opts.configOptions())
	db, err := store.Open(ctx, cfg.Database())
	if err != nil {
		t.Fatal(err)
	}
	states, err := db.JobStates(ctx)
	db.Close()
	if err != nil || !states["morning"].Orphaned {
		t.Fatal(states, err)
	}
	// A hand-added definition clears orphaning and keeps the original anchor.
	j.CreatedBy = "cli"
	if err := jobs.Write(cfg.JobsDir(), j); err != nil {
		t.Fatal(err)
	}
	detail, err = ShowJob(ctx, opts, c, "morning")
	if err != nil || detail.State.Orphaned || !detail.State.AnchorAt.Equal(anchor) {
		t.Fatal(detail, err)
	}
	j.Prompt = "hand edit"
	j.Schedule = "@hourly"
	if err := jobs.Write(cfg.JobsDir(), j); err != nil {
		t.Fatal(err)
	}
	detail, err = ShowJob(ctx, opts, c, "morning")
	if err != nil || detail.Job.Prompt != "hand edit" {
		t.Fatal(detail, err)
	}
}
func TestMCPAndNextValidation(t *testing.T) {
	opts := isolated(t)
	ctx := context.Background()
	c := clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
	if err := Init(ctx, opts); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load(opts.configOptions())
	j := NewJob("agent")
	j.Schedule = "@daily"
	j.Runner = "codex"
	j.Prompt = "hi"
	j.CreatedBy = "mcp"
	if err := jobs.Write(cfg.JobsDir(), j); err != nil {
		t.Fatal(err)
	}
	detail, err := ShowJob(ctx, opts, c, "agent")
	if err != nil || detail.State.Approved || !detail.State.Paused {
		t.Fatal(detail, err)
	}
	for _, tc := range []struct {
		expr, zone string
		count      int
	}{
		{"every 30s", "", 3}, {"@daily", "Mars", 3}, {"", "UTC", 3}, {"@daily", "", 0}, {"@daily", "", 1001},
	} {
		if _, err := NextFires(ctx, opts, c, tc.expr, tc.zone, tc.count); Code(err) != Usage {
			t.Fatal(err)
		}
	}
	if _, err := ShowJob(ctx, opts, c, "../bad"); Code(err) != Usage {
		t.Fatal(err)
	}
	if _, err := AddJob(ctx, opts, c, NewJob("bad"), false); Code(err) != Usage || !strings.Contains(err.Error(), "schedule") {
		t.Fatal(err)
	}
}
