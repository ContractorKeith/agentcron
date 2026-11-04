package app

import (
	"context"
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/store"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInvalidDefinitionClearsFire(t *testing.T) {
	opts := isolated(t)
	ctx := context.Background()
	c := clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
	if err := Init(ctx, opts); err != nil {
		t.Fatal(err)
	}
	j := NewJob("invalid")
	j.Schedule = "every 1m"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hi"
	if _, err := AddJob(ctx, opts, c, j, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opts.ConfigDir, "jobs", "invalid.toml"), []byte("schedule = ["), 0600); err != nil {
		t.Fatal(err)
	}
	views, err := ListJobs(ctx, opts, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || views[0].Status != "invalid" {
		t.Fatalf("views=%+v", views)
	}
	db, err := store.Open(ctx, filepath.Join(opts.DataDir, "agentcron.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	states, err := db.JobStates(ctx)
	if err != nil || states[j.Name].NextFire != nil {
		t.Fatal(states, err)
	}
	detail, err := ShowJob(ctx, opts, c, j.Name)
	if err != nil || detail.Status != "invalid" {
		t.Fatal(detail, err)
	}
}
