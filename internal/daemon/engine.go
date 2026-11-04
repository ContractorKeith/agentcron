package daemon

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

type Executor interface {
	Run(context.Context, jobs.Job, store.Run, bool) (store.Run, error)
}

type Engine struct {
	DB                    *store.Store
	Clock                 clock.Clock
	Runner                Executor
	Sync                  func(context.Context) (map[string]jobs.Job, error)
	Recover               func(context.Context, *store.Run) error
	Concurrency, KeepRuns int
	DataDir               string
	Log                   io.Writer
	wg                    sync.WaitGroup
	mu                    sync.Mutex
	active                map[string]bool
	statuses              map[string]string
}

func (e *Engine) log(v any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.Log != nil {
		_ = json.NewEncoder(e.Log).Encode(v)
	}
}
func (e *Engine) Wait() { e.wg.Wait() }
func (e *Engine) Tick(ctx context.Context) error {
	if err := e.DB.CheckIdentity(); err != nil {
		e.log(map[string]string{"error": err.Error()})
		return err
	}
	if err := e.Reconcile(ctx); err != nil {
		return err
	}
	defs, err := e.Sync(ctx)
	if err != nil {
		return err
	}
	states, err := e.DB.JobStates(ctx)
	if err != nil {
		return err
	}
	if err = e.Enqueue(ctx, defs, states); err != nil {
		return err
	}
	runs, err := e.Claim(ctx, defs, states)
	if err != nil {
		return err
	}
	e.Start(ctx, runs, defs)
	return e.Retention(ctx)
}
func (e *Engine) Start(ctx context.Context, runs []store.Run, defs map[string]jobs.Job) {
	for _, r := range runs {
		e.mu.Lock()
		if e.active == nil {
			e.active = map[string]bool{}
		}
		e.active[r.ID] = true
		e.mu.Unlock()
		e.wg.Add(1)
		go func(r store.Run) {
			defer e.wg.Done()
			result, err := e.Runner.Run(ctx, defs[r.Job], r, true)
			if ctx.Err() != nil && result.State == "cancelled" {
				if stopErr := e.DB.SetDaemonStopped(context.WithoutCancel(ctx), r.ID); stopErr != nil {
					err = stopErr
				}
				result.State = "cancelled"
				result.Error = "daemon stopped"
			}
			e.log(map[string]any{"run": r.ID, "state": result.State, "error": errString(err)})
			e.mu.Lock()
			delete(e.active, r.ID)
			e.mu.Unlock()
		}(r)
	}
}
func errString(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

// Loop responds to reload and otherwise wakes by the next fire, capped at 30s.
func (e *Engine) Loop(ctx context.Context, hup <-chan struct{}) error {
	ctx, cancel := context.WithCancel(ctx)
	defer e.Wait()
	defer cancel()
	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := e.Tick(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		delay := 30 * time.Second
		states, err := e.DB.JobStates(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		for _, s := range states {
			if s.NextFire != nil && !s.Paused && s.Approved && !s.Orphaned {
				d := s.NextFire.Sub(e.Clock.Now())
				if d < delay {
					delay = d
				}
			}
		}
		if delay < time.Second {
			delay = time.Second
		}
		select {
		case <-ctx.Done():
			return nil
		case <-hup:
		case <-e.Clock.After(delay):
		}
	}
}
func (e *Engine) LogJob(name, status string) {
	e.mu.Lock()
	if e.statuses == nil {
		e.statuses = map[string]string{}
	}
	previous := e.statuses[name]
	e.statuses[name] = status
	e.mu.Unlock()
	if previous != status {
		e.log(map[string]string{"job": name, "status": status})
	}
}

func (e *Engine) LogRun(r store.Run) { e.log(r) }
