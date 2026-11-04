package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/ContractorKeith/agentcron/internal/clock"
)

func TestApproveCLI(t *testing.T) {
	root := t.TempDir()
	opts := app.Options{ConfigDir: filepath.Join(root, "config"), DataDir: filepath.Join(root, "data")}
	t.Setenv("AGENTCRON_CONFIG_DIR", opts.ConfigDir)
	t.Setenv("AGENTCRON_DATA_DIR", opts.DataDir)
	if err := app.Init(t.Context(), opts); err != nil {
		t.Fatal(err)
	}
	j := app.NewJob("agent")
	j.Schedule = "@daily"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hi"
	if _, err := app.CreateMCPJob(t.Context(), opts, clock.Real{}, j); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ls", "show"} {
		cmd := newRoot()
		args := []string{name}
		if name == "show" {
			args = append(args, j.Name)
		}
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil || !bytes.Contains(out.Bytes(), []byte("unapproved")) {
			t.Fatal(out.String(), err)
		}
	}
	if err := execute([]string{"approve", "missing"}); app.Code(err) != app.Usage {
		t.Fatal(err)
	}
	if err := execute([]string{"approve", j.Name, "--now", "2026-11-04T12:00:00Z"}); app.Code(err) != app.Usage {
		t.Fatal(err)
	}
	if err := execute([]string{"approve", j.Name}); err != nil {
		t.Fatal(err)
	}
	if err := app.PauseJob(t.Context(), opts, clock.Real{}, j.Name, true); err != nil {
		t.Fatal(err)
	}
	if err := execute([]string{"approve", j.Name}); err != nil {
		t.Fatal(err)
	}
	detail, err := app.ShowJob(t.Context(), opts, clock.Real{}, j.Name)
	if err != nil || !detail.State.Approved || !detail.State.Paused {
		t.Fatal(detail, err)
	}
	// Provenance survives approval; only local state records the human decision.
	raw, err := os.ReadFile(filepath.Join(opts.ConfigDir, "jobs", j.Name+".toml"))
	if err != nil || !bytes.Contains(raw, []byte(`created_by = "mcp"`)) {
		t.Fatal(string(raw), err)
	}
}
