package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/store"
)

// identityLive checks the group leader and its independently recorded start time.
// If launch-time ps failed, the running ledger state plus pgid==pid is the fallback.
func identityLive(r store.Run) bool {
	if r.State != "running" || r.PID == nil {
		return false
	}
	pgid, e := syscall.Getpgid(*r.PID)
	if e != nil || pgid != *r.PID {
		return false
	}
	if r.ProcStart == nil {
		return syscall.Kill(*r.PID, 0) == nil
	}
	start, e := ProcessStart(*r.PID)
	return e == nil && start.Equal(*r.ProcStart)
}
func Cancel(ctx context.Context, db *store.Store, data, id string) error {
	return cancelRun(ctx, db, data, id, clock.Real{}, 10*time.Second)
}
func cancelRun(ctx context.Context, db *store.Store, data, id string, c clock.Clock, grace time.Duration) error {
	r, e := db.GetRun(ctx, id)
	if e != nil {
		return e
	}
	if r.State == "claimed" && r.OwnerPID != nil && r.OwnerStart != nil {
		start, err := processStart(*r.OwnerPID, "/bin/ps")
		if err == nil && start.Equal(*r.OwnerStart) {
			dir := filepath.Join(data, "runs", id)
			if err = os.MkdirAll(dir, 0700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dir, "cancel"), nil, 0600)
		}
	}
	if !identityLive(r) {
		return fmt.Errorf("run is not running or process identity no longer matches run")
	}
	if e = os.WriteFile(filepath.Join(data, "runs", id, "cancel"), nil, 0600); e != nil {
		return e
	}
	pid := *r.PID
	if e = syscall.Kill(-pid, syscall.SIGTERM); e != nil && e != syscall.ESRCH {
		return e
	}
	deadline := c.After(grace)
	for groupExists(pid) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			// Never signal a recycled leader. A vanished leader with remaining group
			// members is safe: its PID cannot be reused while it remains a process group ID.
			if identityLive(r) || syscall.Kill(pid, 0) == syscall.ESRCH {
				_ = syscall.Kill(-pid, syscall.SIGKILL)
			}
			goto finished
		case <-c.After(50 * time.Millisecond):
		}
	}
finished:
	current, e := db.GetRun(ctx, id)
	if e != nil {
		return e
	}
	ownerGone := current.OwnerPID == nil || syscall.Kill(*current.OwnerPID, 0) == syscall.ESRCH
	if current.State == "running" && ownerGone {
		current.State = "cancelled"
		at := c.Now().UTC()
		current.EndedAt = &at
		current.Kept = current.Worktree != ""
		if e = db.FinishRun(context.WithoutCancel(ctx), current); e != nil {
			latest, readErr := db.GetRun(context.WithoutCancel(ctx), id)
			if readErr == nil && store.Terminal(latest.State) {
				return nil
			}
			return e
		}
	}
	return nil
}
