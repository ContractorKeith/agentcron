package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ContractorKeith/agentcron/internal/clock"
)

func TestRunMissingPromptIsRecorded(t *testing.T) {
	ctx := context.Background()
	opts := isolated(t)
	c := clock.Real{}
	if e := Init(ctx, opts); e != nil {
		t.Fatal(e)
	}
	// Publish a valid job, then remove the external prompt before execution.
	dir := filepath.Join(opts.ConfigDir, "jobs")
	prompt := filepath.Join(dir, "prompt.txt")
	if e := os.WriteFile(prompt, []byte("hello"), 0600); e != nil {
		t.Fatal(e)
	}
	j := NewJob("missing-prompt")
	j.Schedule = "@daily"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.PromptFile = "prompt.txt"
	if _, e := AddJob(ctx, opts, c, j, false); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(prompt); e != nil {
		t.Fatal(e)
	}
	r, e := RunJob(ctx, opts, c, j.Name, true, true)
	if Code(e) != RunUnsuccessful || r.State != "blocked_config" || r.Error == "" {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestReaddRetainsLedgerHistory(t *testing.T) {
	ctx := context.Background()
	opts := isolated(t)
	c := clock.Real{}
	if e := Init(ctx, opts); e != nil {
		t.Fatal(e)
	}
	j := NewJob("again")
	j.Schedule = "@daily"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hello"
	if _, e := AddJob(ctx, opts, c, j, false); e != nil {
		t.Fatal(e)
	}
	r, e := RunJob(ctx, opts, c, j.Name, true, true)
	if e != nil {
		t.Fatal(e)
	}
	if e = RemoveJob(ctx, opts, c, j.Name, false); e != nil {
		t.Fatal(e)
	}
	if _, e = AddJob(ctx, opts, c, j, false); e != nil {
		t.Fatal(e)
	}
	detail, e := ShowJob(ctx, opts, c, j.Name)
	if e != nil {
		t.Fatal(e)
	}
	if len(detail.Runs) != 1 || detail.Runs[0].ID != r.ID || detail.State.LastRunID != r.ID {
		t.Fatalf("%+v", detail)
	}
}
