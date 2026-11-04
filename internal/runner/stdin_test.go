package runner

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestLargePromptDoesNotBlockWait(t *testing.T) {
	x, j := setup(t)
	j.Runner = "command"
	j.Command = []string{"claude"}
	j.Prompt = strings.Repeat("p", 1024*1024)
	j.Timeout = "150ms"
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
