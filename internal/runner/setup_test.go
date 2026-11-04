package runner

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestWorktreeSetupFailureIsCleaned(t *testing.T) {
	x, j := setup(t)
	j.Repo = repo(t, x)
	hook := filepath.Join(j.Repo, ".git", "hooks", "post-checkout")
	if e := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0700); e != nil {
		t.Fatal(e)
	}
	r := executeRun(t, x, j, true)
	trees, e := git(context.Background(), j.Repo, "worktree", "list", "--porcelain")
	if e != nil {
		t.Fatal(e)
	}
	branches, e := git(context.Background(), j.Repo, "branch", "--list", "agentcron/*")
	if e != nil {
		t.Fatal(e)
	}
	if r.State != "blocked_config" || r.Kept || strings.Contains(trees, r.ID) || branches != "" {
		t.Fatalf("run=%+v trees=%s branches=%s", r, trees, branches)
	}
}
func TestSetupTimeoutIsBounded(t *testing.T) {
	x, j := setup(t)
	j.Repo = repo(t, x)
	j.Timeout = "1s"
	pidFile := filepath.Join(x.DataDir, "hook-pid")
	hook := filepath.Join(j.Repo, ".git", "hooks", "post-checkout")
	script := "#!/bin/sh\necho $$ > '" + pidFile + "'\nexec sleep 5\n"
	if e := os.WriteFile(hook, []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	defer func() {
		b, _ := os.ReadFile(pidFile)
		pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
		if pid > 0 {
			reapDescendant(pid)
		}
	}()
	start := x.Clock.Now()
	r := executeRun(t, x, j, true)
	if r.State != "timed_out" || !strings.Contains(r.Error, "setup timed out") || x.Clock.Now().Sub(start) >= 5*time.Second {
		t.Fatalf("run=%+v elapsed=%v", r, x.Clock.Now().Sub(start))
	}
	trees, e := git(context.Background(), j.Repo, "worktree", "list", "--porcelain")
	if e != nil || strings.Contains(trees, r.ID) {
		t.Fatalf("worktree survived: %s %v", trees, e)
	}
}
