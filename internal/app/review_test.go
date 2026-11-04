package app

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/config"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func TestMissingJobsDirectory(t *testing.T) {
	opts := isolated(t)
	ctx := context.Background()
	if err := Init(ctx, opts); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load(opts.configOptions())
	if err := os.Remove(cfg.JobsDir()); err != nil {
		t.Fatal(err)
	}
	_, err := ListJobs(ctx, opts, clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)))
	if Code(err) != NotInitialized {
		t.Fatalf("exit=%d err=%v", Code(err), err)
	}
}
func TestPinnedReadsPreserveLedger(t *testing.T) {
	opts := isolated(t)
	ctx := context.Background()
	c := clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
	if err := Init(ctx, opts); err != nil {
		t.Fatal(err)
	}
	j := NewJob("example")
	j.Runner = "claude"
	j.Schedule = "every 2h"
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
		result, err := db.JobStates(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	before := read()
	j.Name = "hand-added"
	if err := jobs.Write(cfg.JobsDir(), j); err != nil {
		t.Fatal(err)
	}
	past := clock.NewFake(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	opts.ReadOnly = true
	if _, err := ListJobs(ctx, opts, past); err != nil {
		t.Fatal(err)
	}
	if _, err := ShowJob(ctx, opts, past, "example"); err != nil {
		t.Fatal(err)
	}
	if _, err := NextFires(ctx, opts, past, "", "", 1); err != nil {
		t.Fatal(err)
	}
	if err := jobs.Remove(cfg.JobsDir(), "example"); err != nil {
		t.Fatal(err)
	}
	if _, err := ListJobs(ctx, opts, past); err != nil {
		t.Fatal(err)
	}
	if after := read(); !reflect.DeepEqual(before, after) {
		t.Fatalf("preview changed state: before=%+v after=%+v", before, after)
	}
}
