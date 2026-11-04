package mcpserver_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/agentcron/internal/app"
)

func TestCreateMemoryDefaultsAndOverrides(t *testing.T) {
	bin := t.TempDir()
	cmd := exec.Command("go", "build", "-o", filepath.Join(bin, "mdkb"), "../../testdata/fakes/mdkb")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake: %s: %v", out, err)
	}
	cs, opts, c := fixture(t, true)
	originalPath := os.Getenv("PATH")
	for _, present := range []bool{false, true} {
		path := t.TempDir()
		if present {
			path = bin
		}
		t.Setenv("PATH", path)
		if present {
			if found, err := exec.LookPath("mdkb"); err != nil || found != filepath.Join(bin, "mdkb") {
				t.Fatal("fake mdkb not first")
			}
		}
		for _, mode := range []string{"default", "memory", "no_memory", "conflict"} {
			name := strings.ReplaceAll(mode, "_", "-")
			if present {
				name += "-present"
			}
			args := map[string]any{"name": name, "schedule": "@daily", "runner": "command", "command": []string{"cat"}, "prompt": "hi"}
			if mode == "memory" || mode == "conflict" {
				args["memory"] = true
			}
			if mode == "no_memory" || mode == "conflict" {
				args["no_memory"] = true
			}
			call(t, cs, "create_job", args, mode == "conflict")
			if mode == "conflict" {
				continue
			}
			detail, err := app.ShowJob(t.Context(), opts, c, name)
			want := mode == "memory" || (mode == "default" && present)
			if err != nil || detail.Job.Memory != want {
				t.Fatal(detail, err)
			}
		}
	}
	t.Setenv("PATH", originalPath)
}
