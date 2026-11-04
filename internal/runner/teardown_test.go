package runner

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCooperativeTimeoutReturnsPromptly(t *testing.T) {
	x, j := setup(t)
	x.Grace = 0
	j.Timeout = "100ms"
	t.Setenv("FAKE_MODE", "sleep")
	start := x.Clock.Now()
	r := executeRun(t, x, j, true)
	if r.State != "timed_out" || x.Clock.Now().Sub(start) >= time.Second {
		t.Fatalf("state=%s elapsed=%s", r.State, x.Clock.Now().Sub(start))
	}
}
func TestDescendantsRemoved(t *testing.T) {
	x, j := setup(t)
	for _, mode := range []string{"background", "spawn-child"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(x.DataDir, "child-"+mode)
			t.Setenv("FAKE_CHILD_PID", path)
			t.Setenv("FAKE_MODE", mode)
			t.Setenv("FAKE_SLEEP", "3s")
			j.Timeout = "150ms"
			if mode == "background" {
				j.Timeout = "5s"
			}
			r := executeRun(t, x, j, true)
			b, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			pid, e := strconv.Atoi(strings.TrimSpace(string(b)))
			if e != nil {
				t.Fatal(e)
			}
			defer func() { _ = syscall.Kill(pid, syscall.SIGKILL); reapDescendant(pid) }()
			// Zombies have exited; reap adopted descendants on Linux before asserting.
			reapDescendant(pid)
			if processLive(pid) {
				t.Fatalf("%s descendant %d survived state %s", mode, pid, r.State)
			}
		})
	}
}
func processLive(pid int) bool {
	out, e := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if e == nil && strings.Contains(string(out), ") Z ") {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func TestTimeoutUsesOneGrace(t *testing.T) {
	x, j := setup(t)
	x.Grace = 400 * time.Millisecond
	j.Timeout = "100ms"
	path := filepath.Join(x.DataDir, "one-grace-child")
	t.Setenv("FAKE_CHILD_PID", path)
	t.Setenv("FAKE_MODE", "spawn-child")
	t.Setenv("FAKE_SLEEP", "3s")
	defer func() {
		b, _ := os.ReadFile(path)
		pid, _ := strconv.Atoi(string(b))
		if pid > 0 {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			reapDescendant(pid)
		}
	}()
	start := x.Clock.Now()
	r := executeRun(t, x, j, true)
	if r.State != "timed_out" || x.Clock.Now().Sub(start) >= 800*time.Millisecond {
		t.Fatalf("state=%s elapsed=%s", r.State, x.Clock.Now().Sub(start))
	}
}
