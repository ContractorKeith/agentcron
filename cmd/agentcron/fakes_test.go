package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFakesRecordOK(t *testing.T) {
	scratch := isolate(t)
	binDir := filepath.Join(scratch, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"claude", "codex", "mdkb"} {
		build(t, filepath.Join(binDir, name), "../../testdata/fakes/"+name)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_MODE", "ok")
	t.Setenv("FAKE_SLEEP", "")
	t.Setenv("AGENTCRON_JOB", "fixture")
	t.Setenv("AGENTCRON_FUTURE_FIELD", "also-recorded")
	t.Setenv("MDKB_ROOT", filepath.Join(scratch, "kb"))
	for _, name := range []string{"claude", "codex", "mdkb"} {
		t.Run(name, func(t *testing.T) {
			record := filepath.Join(scratch, name+".jsonl")
			t.Setenv("FAKE_RECORD", record)
			// Two invocations prove append rather than overwrite.
			for range 2 {
				found, err := exec.LookPath(name)
				if err != nil || found != filepath.Join(binDir, name) {
					t.Fatalf("fake not first: %s %v", found, err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				cmd := testCommand(ctx, name, "--fixture", "two words")
				cmd.Dir = scratch
				cmd.Stdin = strings.NewReader("test prompt\n")
				out, err := cmd.CombinedOutput()
				cancel()
				if err != nil || string(out) != "DONE\n" {
					t.Fatalf("fake output %q: %v", out, err)
				}
			}
			contents, err := os.ReadFile(record)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(contents)), "\n")
			if len(lines) != 2 {
				t.Fatalf("records: %s", contents)
			}
			for _, line := range lines {
				var got struct {
					Argv  []string
					Cwd   string
					Env   map[string]string
					Stdin string
				}
				if err := json.Unmarshal([]byte(line), &got); err != nil {
					t.Fatal(err)
				}
				wantArgv := []string{name, "--fixture", "two words"}
				cwd, err := filepath.EvalSymlinks(scratch)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got.Argv, wantArgv) || got.Cwd != cwd || got.Stdin != "test prompt\n" {
					t.Fatalf("record: %+v", got)
				}
				for _, key := range []string{"AGENTCRON_JOB", "AGENTCRON_FUTURE_FIELD", "MDKB_ROOT"} {
					if got.Env[key] != os.Getenv(key) {
						t.Fatalf("env %s: %v", key, got.Env)
					}
				}
			}
		})
	}
}
