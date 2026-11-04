package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestLargePromptDoesNotBlockWait(t *testing.T) {
	x, j := setup(t)
	j.Runner = "command"
	// Model macOS's first-exec latency before the fake starts.
	delayed := filepath.Join(x.DataDir, "cold-claude")
	binary, err := exec.LookPath("claude")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(delayed, []byte("#!/bin/sh\nsleep 0.2\nexec '"+binary+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	j.Command = []string{delayed}
	j.Prompt = strings.Repeat("p", 1024*1024)
	j.Timeout = "1s"
	path := filepath.Join(x.DataDir, "stdin-child")
	t.Setenv("FAKE_CHILD_PID", path)
	t.Setenv("FAKE_MODE", "background")
	t.Setenv("FAKE_SLEEP", "3s")
	defer func() {
		b, _ := os.ReadFile(path)
		pid, _ := strconv.Atoi(string(b))
		if pid > 0 {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			reapDescendant(pid)
		}
	}()
	r := executeRun(t, x, j, true)
	if r.State != "succeeded" {
		t.Fatalf("large unread stdin left run %s", r.State)
	}
	b, e := os.ReadFile(filepath.Join(x.DataDir, "runs", r.ID, "prompt.txt"))
	if e != nil || !strings.HasSuffix(string(b), j.Prompt) {
		t.Fatalf("prompt file: %v", e)
	}
}
