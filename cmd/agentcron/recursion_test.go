package main

import (
	"github.com/ContractorKeith/agentcron/internal/app"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIRecursion(t *testing.T) {
	isolate(t)
	if _, err := cli(t, "init"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTCRON_RUN_ID", "parent")
	for _, args := range [][]string{{"add", "nested", "--schedule", "@daily", "--prompt", "hi"}, {"rm", "missing"}, {"pause", "missing"}, {"resume", "missing"}, {"run", "missing"}, {"run", "missing", "--now"}, {"approve", "missing"}} {
		_, err := cli(t, args...)
		if app.Code(err) != app.Refused {
			t.Errorf("%v: %v", args, err)
		}
	}
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "agentcron", "config.toml")
	if err := os.WriteFile(cfg, []byte("allow_agent_scheduling = true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := cli(t, "approve", "missing")
	if app.Code(err) != app.Refused || !strings.Contains(err.Error(), "human") {
		t.Fatal(err)
	}
}
