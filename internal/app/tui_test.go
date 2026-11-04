package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/daemon"
)

func TestTUIAndBoundedLogTail(t *testing.T) {
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
	api := TUI{Options: opts, Clock: c}
	if err := api.Enqueue(ctx, "daily"); err != nil {
		t.Fatal(err)
	}
	runs, err := api.Runs(ctx, "daily")
	if err != nil || len(runs) != 1 || runs[0].State != "queued" {
		t.Fatalf("%v %v", runs, err)
	}
	if err := api.Pause(ctx, "daily", true); err != nil {
		t.Fatal(err)
	}
	js, err := api.Jobs(ctx)
	if err != nil || len(js) != 1 || !js[0].State.Paused {
		t.Fatalf("%v %v", js, err)
	}
	if err := api.Pause(ctx, "daily", false); err != nil {
		t.Fatal(err)
	}
	running, err := api.DaemonRunning(ctx)
	if err != nil || running {
		t.Fatalf("%v %v", running, err)
	}
	lock, err := daemon.Lock(opts.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if running, err = api.DaemonRunning(ctx); err != nil || !running {
		t.Fatalf("%v %v", running, err)
	}
	id := runs[0].ID
	if got, err := api.Tail(ctx, id, false); err != nil || got != "" {
		t.Fatalf("%q %v", got, err)
	}
	dir := filepath.Join(opts.DataDir, "runs", id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	output := strings.Repeat("x", 100000) + "END"
	if err := os.WriteFile(filepath.Join(dir, "stdout.log"), []byte(output), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := api.Tail(ctx, id, false)
	if err != nil || len(got) != 64*1024 || !strings.HasSuffix(got, "END") {
		t.Fatalf("tail size %d: %v", len(got), err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stderr.log"), []byte("stderr only"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := api.Tail(ctx, id, true); err != nil || got != "stderr only" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := api.Tail(ctx, "../../unknown", false); err == nil {
		t.Fatal("unknown run accepted")
	}
}
