package runner

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/store"
)

func TestCancelAndOverlap(t *testing.T) {
	x, j := setup(t)
	t.Setenv("FAKE_MODE", "ignore-term")
	t.Setenv("FAKE_RECORD", filepath.Join(x.DataDir, "started"))
	r, e := store.NewRun(j.Name, "manual", x.Clock.Now())
	if e != nil {
		t.Fatal(e)
	}
	if e = x.DB.CreateRun(context.Background(), r); e != nil {
		t.Fatal(e)
	}
	type outcome struct {
		r store.Run
		e error
	}
	done := make(chan outcome, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { r, e := x.Run(ctx, j, r, true); done <- outcome{r, e} }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("runner did not stop")
		}
	})
	started := false
	for range 250 {
		current, err := x.DB.GetRun(context.Background(), r.ID)
		if err != nil {
			t.Fatal(err)
		}
		if current.State == "running" {
			if _, err = os.Stat(filepath.Join(x.DataDir, "started")); err == nil {
				started = true
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !started {
		t.Fatal("runner did not start")
	}
	second := executeRun(t, x, j, true)
	if second.State != "skipped" {
		t.Fatal(second)
	}
	if e = Cancel(context.Background(), x.DB, x.DataDir, r.ID); e != nil {
		t.Fatal(e)
	}
	got := <-done
	done <- got // Leave a completion for cleanup to join.
	if got.e != nil || got.r.State != "cancelled" {
		t.Fatalf("%+v %v", got.r, got.e)
	}
	if e = syscall.Kill(*got.r.PID, 0); e == nil {
		t.Fatal("cancelled process survived")
	}
	if e = Cancel(context.Background(), x.DB, x.DataDir, r.ID); e == nil {
		t.Fatal("terminal run cancellation accepted")
	}
}
func TestCancelRejectsStaleIdentity(t *testing.T) {
	x, _ := setup(t)
	r, e := store.NewRun("fixture", "manual", x.Clock.Now())
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	if e = x.DB.CreateRun(ctx, r); e != nil {
		t.Fatal(e)
	}
	if e = x.DB.ClaimRun(ctx, r.ID); e != nil {
		t.Fatal(e)
	}
	// An unrelated process is never signalled: its start precedes the future timestamp.
	if e = x.DB.StartRun(ctx, r.ID, os.Getpid(), x.Clock.Now().Add(time.Hour), ""); e != nil {
		t.Fatal(e)
	}
	if e = Cancel(ctx, x.DB, x.DataDir, r.ID); e == nil {
		t.Fatal("stale identity accepted")
	}
}

func TestContextCancel(t *testing.T) {
	x, j := setup(t)
	t.Setenv("FAKE_MODE", "ignore-term")
	r, e := store.NewRun(j.Name, "manual", x.Clock.Now())
	if e != nil {
		t.Fatal(e)
	}
	if e = x.DB.CreateRun(context.Background(), r); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, e = x.Run(ctx, j, r, true)
	if e != nil || r.State != "cancelled" {
		t.Fatalf("%+v %v", r, e)
	}
	if e = syscall.Kill(*r.PID, 0); e == nil {
		t.Fatal("child survived")
	}
}
