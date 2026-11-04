package app

import (
	"context"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/config"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func TestDryRunsPreserveState(t *testing.T) {
	opts := isolated(t)
	ctx := context.Background()
	c := clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
	if err := Init(ctx, opts); err != nil {
		t.Fatal(err)
	}
	j := NewJob("existing")
	j.Schedule = "@daily"
	j.Runner = "claude"
	j.Prompt = "hi"
	if _, err := AddJob(ctx, opts, c, j, false); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load(opts.configOptions())
	read := func() map[string]store.JobState {
		t.Helper()
		db, err := store.OpenExisting(ctx, cfg.Database())
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		states, err := db.JobStates(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return states
	}
	before := read()
	c.Advance(time.Hour)
	if err := RemoveJob(ctx, opts, c, "existing", true); err != nil {
		t.Fatal(err)
	}
	j.Name = "preview"
	if _, err := AddJob(ctx, opts, c, j, true); err != nil {
		t.Fatal(err)
	}
	after := read()
	if len(after) != len(before) || !after["existing"].UpdatedAt.Equal(before["existing"].UpdatedAt) {
		t.Fatal(before, after)
	}
}
