package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func TestCancelUnknownReturnsUsage(t *testing.T) {
	opts := isolated(t)
	ctx := context.Background()
	if e := Init(ctx, opts); e != nil {
		t.Fatal(e)
	}
	if e := CancelRun(ctx, opts, clock.Real{}, "missing"); Code(e) != Usage {
		t.Fatalf("code=%d err=%v", Code(e), e)
	}
}
func TestWaitWithoutInlineIsRefused(t *testing.T) {
	opts := isolated(t)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	c := clock.Real{}
	if e := Init(ctx, opts); e != nil {
		t.Fatal(e)
	}
	j := NewJob("queue")
	j.Schedule = "@daily"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hello"
	if _, e := AddJob(ctx, opts, c, j, false); e != nil {
		t.Fatal(e)
	}
	_, e := RunJob(ctx, opts, c, j.Name, false, true)
	if Code(e) != Usage || !strings.Contains(e.Error(), "the daemon executes queued runs; use --now to run inline") {
		t.Fatalf("%v", e)
	}
}
func TestPruneContinuesPastUnexpectedPath(t *testing.T) {
	opts := isolated(t)
	ctx := context.Background()
	c := clock.Real{}
	if e := Init(ctx, opts); e != nil {
		t.Fatal(e)
	}
	db, e := store.Open(ctx, filepath.Join(opts.DataDir, "agentcron.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var valid store.Run
	for _, bad := range []bool{false, true} {
		r, e := store.NewRun("prune", "manual", c.Now())
		if e != nil {
			t.Fatal(e)
		}
		if e = db.CreateRun(ctx, r); e != nil {
			t.Fatal(e)
		}
		r.Worktree = filepath.Join(opts.DataDir, "worktrees", r.Job, r.ID)
		if bad {
			r.Worktree = filepath.Join(opts.DataDir, "unexpected")
		}
		if e = os.MkdirAll(r.Worktree, 0700); e != nil {
			t.Fatal(e)
		}
		r.State = "skipped"
		r.Kept = true
		at := c.Now()
		r.EndedAt = &at
		if e = db.FinishRun(ctx, r); e != nil {
			t.Fatal(e)
		}
		if !bad {
			valid = r
		}
	}
	report, e := Prune(ctx, opts, c, false)
	if _, statErr := os.Stat(valid.Worktree); !os.IsNotExist(statErr) {
		t.Fatalf("valid worktree not pruned: %v report=%v err=%v", statErr, report, e)
	}
	if e == nil || !strings.Contains(e.Error(), "unexpected worktree path") {
		t.Fatalf("missing report: %v", e)
	}
}
