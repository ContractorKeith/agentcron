package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/config"
	"github.com/ContractorKeith/agentcron/internal/daemon"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/runner"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func Schedule(ctx context.Context, opts Options, c clock.Clock, loop bool, noWait bool, hup <-chan struct{}, log io.Writer) error {
	if opts.ReadOnly {
		return &Error{Usage, errors.New("now preview is read-only")}
	}
	cfg, err := config.Load(opts.configOptions())
	if err != nil {
		return err
	}
	if _, err = os.Stat(cfg.Database()); os.IsNotExist(err) {
		return &Error{NotInitialized, fmt.Errorf("not initialised; run agentcron init")}
	}
	var lock *os.File
	if os.Getenv("AGENTCRON_TICK_WORKER") == "1" {
		lock = os.NewFile(3, "daemon.lock")
		if err = lock.Truncate(0); err == nil {
			_, err = lock.WriteAt([]byte(fmt.Sprintln(os.Getpid())), 0)
		}
	} else {
		lock, err = daemon.Lock(cfg.DataDir)
	}
	if err != nil {
		var locked *daemon.LockedError
		if errors.As(err, &locked) {
			return &Error{Locked, err}
		}
		return err
	}
	defer lock.Close()
	if noWait {
		binary, err := os.Executable()
		if err != nil {
			return err
		}
		cmd := exec.Command(binary, "--config", cfg.ConfigDir, "--data-dir", cfg.DataDir, "tick")
		cmd.Env = append(os.Environ(), "AGENTCRON_TICK_WORKER=1")
		cmd.ExtraFiles = []*os.File{lock}
		workerLog, err := os.OpenFile(filepath.Join(cfg.DataDir, "daemon.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer workerLog.Close()
		cmd.Stderr = workerLog
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err = cmd.Start(); err != nil {
			return err
		}
		return cmd.Process.Release()
	}

	// Reconciliation precedes definition sync.
	db, err := store.Open(ctx, cfg.Database())
	if err != nil {
		return err
	}
	defer db.Close()
	session := &jobSession{config: cfg, db: db}
	engine := &daemon.Engine{DB: db, Clock: c, DataDir: cfg.DataDir, Concurrency: cfg.Concurrency, KeepRuns: cfg.KeepRuns, Log: log}
	engine.Runner = runner.Runner{Observe: engine.LogRun, DB: db, Clock: c, DataDir: cfg.DataDir, JobsDir: cfg.JobsDir(), ConfigDir: cfg.ConfigDir}
	engine.Sync = func(ctx context.Context) (map[string]jobs.Job, error) {
		session.now = c.Now().UTC().Truncate(time.Second)
		if err := session.sync(ctx, true); err != nil {
			return nil, err
		}
		defs := map[string]jobs.Job{}
		for _, v := range session.views {
			if v.Job != nil {
				defs[v.Name] = *v.Job
			}
			engine.LogJob(v.Name, v.Status)
		}
		return defs, nil
	}
	engine.Recover = func(ctx context.Context, r *store.Run) error {
		j, err := jobs.ReadForRun(cfg.JobsDir(), r.Job)
		if err != nil {
			return nil
		}
		if _, err = os.Stat(r.Worktree); os.IsNotExist(err) {
			return nil
		}
		return runner.RecoverClaimed(ctx, j, r)
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if loop {
		return engine.Loop(runCtx, hup)
	}
	if err = engine.Tick(runCtx); err != nil {
		cancel()
		engine.Wait()
		return err
	}
	// A one-pass process cannot safely leave in-process runner goroutines behind.
	// no-wait is launched as a detached worker by the CLI before entering here.
	engine.Wait()
	return engine.Retention(context.WithoutCancel(ctx))
}
