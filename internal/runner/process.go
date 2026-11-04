package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
)

func lock(data, job string) (*os.File, bool, error) {
	if err := os.MkdirAll(filepath.Join(data, "runs"), 0700); err != nil {
		return nil, false, err
	}
	f, err := os.OpenFile(filepath.Join(data, "runs", ".lock-"+job), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, false, err
	}
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, false, nil
		}
		return nil, false, err
	}
	return f, true, nil
}
func groupExists(pid int) bool { return syscall.Kill(-pid, 0) == nil }

// teardownGroup returns immediately when the group is gone. Descendants get a
// bounded TERM grace even after their leader has already been reaped.
func teardownGroup(pid int, c clock.Clock, grace time.Duration) {
	if !groupExists(pid) {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	waitGroup(pid, c, c.After(grace))
}
func waitGroup(pid int, c clock.Clock, deadline <-chan time.Time) {
	for groupExists(pid) {
		select {
		case <-deadline:
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			return
		case <-c.After(50 * time.Millisecond):
		}
	}
}
func terminate(pid int, done <-chan error, c clock.Clock, grace time.Duration) error {
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	deadline := c.After(grace)
	select {
	case result := <-done:
		if groupExists(pid) {
			_ = syscall.Kill(-pid, syscall.SIGTERM)
			waitGroup(pid, c, deadline)
		}
		return result
	case <-deadline:
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		return <-done
	}
}

// ProcessStart uses ps on both Linux and macOS (macOS has no /proc). ps lstart
// has second precision. Record the observed process start in proc_start and require
// equality before signalling: a later recycled PID must also be refused.
func ProcessStart(pid int) (time.Time, error) {
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "lstart=")
	cmd.Env = append(os.Environ(), "LC_ALL=C", "TZ=UTC")
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse("Mon Jan 2 15:04:05 2006", strings.Join(strings.Fields(string(out)), " "))
}
