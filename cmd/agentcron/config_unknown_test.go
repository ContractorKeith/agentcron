package main

import (
	"github.com/ContractorKeith/agentcron/internal/app"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnknownConfigKey(t *testing.T) {
	scratch := isolate(t)
	if _, err := cli(t, "init"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scratch, "config", "agentcron", "config.toml"), []byte("keep_run = 5\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := cli(t, "ls")
	if app.Code(err) != app.Usage || !strings.Contains(err.Error(), "keep_run") {
		t.Fatal(err)
	}
}
