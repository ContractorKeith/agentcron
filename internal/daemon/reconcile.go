package daemon

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/runner"
	"github.com/ContractorKeith/agentcron/internal/store"
)

// Process seams let restart rules use fake identities without starting processes.
type Processes struct {
	GroupExists func(int) bool
	Alive       func(int) bool
	Start       func(int) (time.Time, error)
	Kill        func(int) error
}

func systemProcesses(c clock.Clock) Processes {
	return Processes{
		Alive:       func(pid int) bool { return syscall.Kill(pid, 0) != syscall.ESRCH },
		Start:       runner.ProcessStart,
		Kill:        func(pid int) error { runner.TeardownGroup(pid, c, 10*time.Second); return nil },
		GroupExists: func(pid int) bool { return syscall.Kill(-pid, 0) == nil },
	}
}
func (e *Engine) Reconcile(ctx context.Context) error {
	return e.reconcile(ctx, systemProcesses(e.Clock))
}
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
		ownerLive := false
		if r.OwnerPID != nil && r.OwnerStart != nil && p.Alive(*r.OwnerPID) {
			start, err := p.Start(*r.OwnerPID)
			ownerLive = err == nil && start.Equal(*r.OwnerStart)
		}
		live := false
		if r.PID != nil && p.Alive(*r.PID) {
			start, err := p.Start(*r.PID)
			live = err == nil && (r.ProcStart == nil || start.Equal(*r.ProcStart)) && (r.StartedAt == nil || !start.After(*r.StartedAt))
		}
		if ownerLive {
			continue
		}
		// Never adopt an orphan: tear down only a matching process identity.
		remainingGroup := r.PID != nil && !p.Alive(*r.PID) && p.GroupExists != nil && p.GroupExists(*r.PID)
		if live || remainingGroup {
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
			current, readErr := e.DB.GetRun(ctx, r.ID)
			if readErr == nil && store.Terminal(current.State) {
				continue
			}
			return err
		}
		r.State = "unknown"
		e.log(r)
	}
	return nil
}
