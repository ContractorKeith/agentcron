package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ContractorKeith/agentcron/internal/runner"
)

// Process seams let restart rules use fake identities without starting processes.
type Processes struct {
	Alive func(int) bool
	Start func(int) (time.Time, error)
	Kill  func(int) error
}

func systemProcesses() Processes {
	return Processes{
		Alive: func(pid int) bool { return syscall.Kill(pid, 0) != syscall.ESRCH },
		Start: func(pid int) (time.Time, error) {
			pgid, err := syscall.Getpgid(pid)
			if err != nil {
				return time.Time{}, err
			}
			if pgid != pid {
				return time.Time{}, fmt.Errorf("process group identity changed")
			}
			return runner.ProcessStart(pid)
		},
		Kill: func(pid int) error { return syscall.Kill(-pid, syscall.SIGKILL) },
	}
}
func (e *Engine) Reconcile(ctx context.Context) error { return e.reconcile(ctx, systemProcesses()) }
func (e *Engine) reconcile(ctx context.Context, p Processes) error {
	runs, err := e.DB.ListRuns(ctx, "", "", -1)
	if err != nil {
		return err
	}
	for _, r := range runs {
		if r.State != "claimed" && r.State != "running" {
			continue
		}
		e.mu.Lock()
		owned := e.active[r.ID]
		e.mu.Unlock()
		if owned {
			continue
		}
		ownerLive := r.OwnerPID != nil && p.Alive(*r.OwnerPID)
		live := false
		if r.PID != nil && p.Alive(*r.PID) {
			start, err := p.Start(*r.PID)
			live = err == nil && (r.ProcStart == nil || start.Equal(*r.ProcStart)) && (r.StartedAt == nil || !start.After(*r.StartedAt))
		}
		if ownerLive && (r.State == "claimed" || live) {
			continue
		}
		// Never adopt an orphan: tear down only a matching process identity.
		if live {
			if err := p.Kill(*r.PID); err != nil && err != syscall.ESRCH {
				return err
			}
		}
		at := e.Clock.Now()
		r.EndedAt = &at
		r.Error = "owner or process vanished; outcome unknown"
		if r.Worktree == "" {
			r.Worktree = filepath.Join(e.DataDir, "worktrees", r.Job, r.ID)
		}
		if _, err := os.Stat(r.Worktree); err == nil {
			r.Kept = true
		}
		if r.State == "claimed" && e.Recover != nil {
			if err := e.Recover(ctx, &r); err != nil {
				return err
			}
		}
		if err := e.DB.Unknown(ctx, r); err != nil {
			return err
		}
		r.State = "unknown"
		e.log(r)
	}
	return nil
}
