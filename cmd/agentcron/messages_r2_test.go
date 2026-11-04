package main

import (
	"github.com/ContractorKeith/agentcron/internal/app"
	"os"
	"path/filepath"
	"testing"
)

func TestSchedulingAndJobMessages(t *testing.T) {
	root := isolate(t)
	if _, err := cli(t, "init"); err != nil {
		t.Fatal(err)
	}
	_, err := cli(t, "run", "missing")
	if app.Code(err) != app.Usage || err.Error() != `unknown job "missing"` {
		t.Errorf("missing: %v", err)
	}
	path := filepath.Join(root, "config", "agentcron", "jobs", "Morning.toml")
	if err := os.WriteFile(path, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = cli(t, "add", "morning", "--schedule", "@daily", "--runner", "command", "--command", "cat", "--prompt", "hi", "--no-memory")
	if app.Code(err) != app.Usage || err.Error() != `a job file differing only in case exists: Morning.toml` {
		t.Errorf("case: %v", err)
	}
	t.Setenv("AGENTCRON_RUN_ID", "parent")
	_, err = cli(t, "run", "missing")
	if app.Code(err) != app.Refused || err.Error() != "Scheduling from inside a run requires allow_agent_scheduling = true in config.toml" {
		t.Errorf("guard: %v", err)
	}
}
