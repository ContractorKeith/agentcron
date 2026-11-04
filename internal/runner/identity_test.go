package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/store"
)

func TestCancelWithMissingLaunchIdentity(t *testing.T) {
	x, j := setup(t)
	t.Setenv("FAKE_MODE", "ignore-term")
	if e := os.WriteFile(filepath.Join(x.DataDir, "bin", "ps"), []byte("#!/bin/sh\nexit 1\n"), 0700); e != nil {
		t.Fatal(e)
	}
	r, e := store.NewRun(j.Name, "manual", x.Clock.Now())
	if e != nil {
		t.Fatal(e)
	}
	if e = x.DB.CreateRun(context.Background(), r); e != nil {
		t.Fatal(e)
	}
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	done := make(chan error, 1)
	go func() { _, e := x.Run(ctx, j, r, true); done <- e }()
	defer func() { stop(); <-done }()
	var running store.Run
	for range 500 {
		running, e = x.DB.GetRun(context.Background(), r.ID)
		if e != nil {
			t.Fatal(e)
		}
		if running.State == "running" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if running.State != "running" || running.ProcStart != nil {
		t.Fatalf("%+v", running)
	}
	if e = cancelRun(context.Background(), x.DB, x.DataDir, r.ID, x.Clock, x.Grace); e != nil {
		t.Fatal(e)
	}
	e = <-done
	done <- e
	if e != nil {
		t.Fatal(e)
	}
	got, e := x.DB.GetRun(context.Background(), r.ID)
	if e != nil || got.State != "cancelled" {
		t.Fatalf("%+v %v", got, e)
	}
}
