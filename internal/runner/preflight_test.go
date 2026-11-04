package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ContractorKeith/agentcron/internal/memory"
	"github.com/ContractorKeith/agentcron/internal/store"
	"time"
)

func TestBlockedConfig(t *testing.T) {
	x, base := setup(t)
	repository := repo(t, x)
	for _, reason := range []string{"missing-runner", "missing-command", "missing-repo", "not-repo", "bad-base", "missing-prompt", "unapproved"} {
		t.Run(reason, func(t *testing.T) {
			j := base
			approved := true
			switch reason {
			case "missing-runner":
				t.Setenv("PATH", x.JobsDir)
			case "missing-command":
				j.Runner = "command"
				j.Command = []string{"agentcron-does-not-exist"}
			case "missing-repo":
				j.Repo = filepath.Join(x.DataDir, "missing")
			case "not-repo":
				j.Repo = x.DataDir
			case "bad-base":
				j.Repo = repository
				j.Base = "nonexistent"
			case "missing-prompt":
				j.Prompt = ""
				j.PromptFile = "missing.txt"
			case "unapproved":
				approved = false
			}
			record := filepath.Join(x.DataDir, "blocked-"+reason)
			t.Setenv("FAKE_RECORD", record)
			r := executeRun(t, x, j, approved)
			if r.State != "blocked_config" || r.Error == "" || r.PID != nil {
				t.Fatalf("%+v", r)
			}
			if _, e := os.Stat(record); !os.IsNotExist(e) {
				t.Fatal("runner launched")
			}
		})
	}
}
func TestDefaultBase(t *testing.T) {
	x, j := setup(t)
	j.Repo = repo(t, x)
	head, e := git(context.Background(), j.Repo, "rev-parse", "HEAD")
	if e != nil {
		t.Fatal(e)
	}
	p, e := preflight(context.Background(), j, true, x.JobsDir)
	if e != nil || p.base != head {
		t.Fatalf("%+v %v", p, e)
	}
	for _, args := range [][]string{{"update-ref", "refs/remotes/origin/main", head}, {"symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main"}, {"commit", "--allow-empty", "-m", "ahead"}} {
		if _, e = git(context.Background(), j.Repo, args...); e != nil {
			t.Fatal(e)
		}
	}
	p, e = preflight(context.Background(), j, true, x.JobsDir)
	if e != nil || p.base != head {
		t.Fatalf("%+v %v", p, e)
	}
}
func TestPreambleGolden(t *testing.T) {
	want, err := os.ReadFile("../../testdata/golden/preamble.txt")
	if err != nil {
		t.Fatal(err)
	}
	r := store.Run{ID: "r_20261104T120000Z_ABCD", Job: "fixture", Trigger: "manual", ScheduledFor: time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)}
	if got := Preamble(r, nil, "/data", memory.Preamble("fixture", false)); got != string(want)+"\n" {
		t.Fatalf("preamble %q", got)
	}
}
