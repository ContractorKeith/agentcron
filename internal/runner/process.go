package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/store"
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
func terminate(pid int, done <-chan error, c clock.Clock, grace time.Duration) error {
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	// Even if the leader exits, descendants may ignore TERM. Always kill the group
	// after the grace period; the leader is reaped by the caller's Wait goroutine.
	var result error
	deadline := c.After(grace)
	select {
	case result = <-done:
	case <-deadline:
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		return <-done
	}
	<-deadline
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	return result
}

// ProcessStart uses ps on both Linux and macOS (macOS has no /proc). ps lstart
// has second precision. Record the observed process start in started_at and require
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
func Cancel(ctx context.Context, db *store.Store, data, id string) error {
	r, err := db.GetRun(ctx, id)
	if err != nil {
		return err
	}
	if r.State != "running" || r.PID == nil || r.StartedAt == nil {
		return fmt.Errorf("run is not running")
	}
	start, err := ProcessStart(*r.PID)
	if err != nil {
		return err
	}
	pgid, err := syscall.Getpgid(*r.PID)
	if err != nil {
		return err
	}
	if !start.Equal(*r.StartedAt) || pgid != *r.PID {
		return fmt.Errorf("process identity no longer matches run")
	}
	// Serialize cancellation with finalization through the request file. The inline
	// owner performs escalation and records cancelled, including a fast TERM exit.
	if err = os.WriteFile(filepath.Join(data, "runs", id, "cancel"), nil, 0600); err != nil {
		return err
	}
	return syscall.Kill(-pgid, syscall.SIGTERM)
}
