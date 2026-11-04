package app

import (
	"github.com/ContractorKeith/agentcron/internal/clock"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPruneKeepsDirtyAndAhead(t *testing.T) {
	for _, kind := range []string{"dirty", "ahead"} {
		t.Run(kind, func(t *testing.T) {
			opts := isolated(t)
			c := clock.Real{}
			if err := Init(t.Context(), opts); err != nil {
				t.Fatal(err)
			}
			repo := t.TempDir()
			git := func(args ...string) {
				t.Helper()
				cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
				cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatal(string(out), err)
				}
			}
			git("init")
			git("commit", "--allow-empty", "-m", "initial")
			j := NewJob("kept")
			j.Schedule = "@daily"
			j.Runner = "command"
			j.Prompt = "hi"
			j.Repo = repo
			j.Memory = false
			if kind == "dirty" {
				j.Command = []string{"touch", "result"}
			} else {
				j.Command = []string{"git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "run"}
			}
			if _, err := AddJob(t.Context(), opts, c, j, false); err != nil {
				t.Fatal(err)
			}
			r, err := RunJob(t.Context(), opts, c, j.Name, true, true)
			if err != nil || !r.Kept {
				t.Fatal(r, err)
			}
			report, err := Prune(t.Context(), opts, c, false, false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = os.Stat(r.Worktree); err != nil {
				t.Fatalf("destroyed %s worktree: %v report=%v", kind, err, report)
			}
			if kind == "dirty" {
				if _, err = os.Stat(filepath.Join(r.Worktree, "result")); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Prune(t.Context(), opts, c, false, true); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(r.Worktree); !os.IsNotExist(err) {
				t.Fatal("force left worktree", err)
			}
		})
	}
}
