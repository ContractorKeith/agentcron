package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/store"
)

type notifyWriter struct {
	buffer bytes.Buffer
	first  chan struct{}
}

func (w *notifyWriter) Write(p []byte) (int, error) {
	n, e := w.buffer.Write(p)
	select {
	case w.first <- struct{}{}:
	default:
	}
	return n, e
}
func TestLogsFollowDrainsTerminalOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := isolated(t)
	c := clock.Real{}
	if e := Init(ctx, opts); e != nil {
		t.Fatal(e)
	}
	db, e := store.Open(ctx, filepath.Join(opts.DataDir, "agentcron.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	r, e := store.NewRun("logs", "manual", c.Now())
	if e != nil {
		t.Fatal(e)
	}
	if e = db.CreateRun(ctx, r); e != nil {
		t.Fatal(e)
	}
	if e = db.ClaimRun(ctx, r.ID); e != nil {
		t.Fatal(e)
	}
	if e = db.StartRun(ctx, r.ID, 123, c.Now(), ""); e != nil {
		t.Fatal(e)
	}
	dir := filepath.Join(opts.DataDir, "runs", r.ID)
	if e = os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "stdout.log")
	if e = os.WriteFile(path, []byte("first\n"), 0600); e != nil {
		t.Fatal(e)
	}
	out := &notifyWriter{first: make(chan struct{}, 1)}
	done := make(chan error, 1)
	go func() { done <- Logs(ctx, opts, c, r.ID, false, true, out) }()
	// Always join the follower before its output is inspected or the database closes.
	defer func() { cancel(); <-done }()
	select {
	case <-out.first:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if e = os.WriteFile(path, []byte("first\nlast\n"), 0600); e != nil {
		t.Fatal(e)
	}
	r.State = "succeeded"
	at := c.Now()
	r.EndedAt = &at
	if e = db.FinishRun(ctx, r); e != nil {
		t.Fatal(e)
	}
	e = <-done
	done <- e
	if e != nil || out.buffer.String() != "first\nlast\n" {
		t.Fatalf("output=%q err=%v", out.buffer.String(), e)
	}
}
func TestPruneAfterRemovingJob(t *testing.T) {
	ctx := context.Background()
	opts := isolated(t)
	c := clock.Real{}
	if e := Init(ctx, opts); e != nil {
		t.Fatal(e)
	}
	j := NewJob("kept")
	j.Schedule = "@daily"
	j.Runner = "command"
	j.Command = []string{"touch", "result"}
	j.Prompt = "hello"
	if _, e := AddJob(ctx, opts, c, j, false); e != nil {
		t.Fatal(e)
	}
	r, e := RunJob(ctx, opts, c, j.Name, true, true)
	if e != nil || !r.Kept {
		t.Fatalf("%+v %v", r, e)
	}
	if e = RemoveJob(ctx, opts, c, j.Name, false); e != nil {
		t.Fatal(e)
	}
	if _, e = Prune(ctx, opts, c, true, false); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(r.Worktree); e != nil {
		t.Fatal("dry run removed scratch")
	}
	if _, e = Prune(ctx, opts, c, false, true); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(r.Worktree); !os.IsNotExist(e) {
		t.Fatal("prune left scratch")
	}
	kept, e := Worktrees(ctx, opts, c)
	if e != nil || len(kept) != 0 {
		t.Fatalf("%+v %v", kept, e)
	}
}
