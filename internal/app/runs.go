package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/runner"
	"github.com/ContractorKeith/agentcron/internal/store"
)

type Run = store.Run

func RunJob(ctx context.Context, opts Options, c clock.Clock, name string, now, wait bool) (Run, error) {
	if opts.ReadOnly {
		return Run{}, &Error{Usage, fmt.Errorf("now preview is read-only")}
	}
	s, err := openJobs(ctx, opts, c, false)
	if err != nil {
		return Run{}, err
	}
	defer s.db.Close()
	j, err := jobs.ReadForRun(s.config.JobsDir(), name)
	if err != nil {
		return Run{}, &Error{Usage, err}
	}
	states, err := s.db.JobStates(ctx)
	if err != nil {
		return Run{}, err
	}
	state, exists := states[name]
	approved := state.Approved
	if !exists {
		approved = j.CreatedBy == "cli"
	}
	if !approved {
		return Run{}, &Error{Refused, fmt.Errorf("job %q is unapproved; run agentcron approve %s first", name, name)}
	}
	r, err := store.NewRun(name, "manual", c.Now())
	if err != nil {
		return r, err
	}
	if err = s.db.CreateRun(ctx, r); err != nil {
		return r, err
	}
	if now {
		r, err = (runner.Runner{DB: s.db, Clock: c, DataDir: s.config.DataDir, JobsDir: s.config.JobsDir(), ConfigDir: s.config.ConfigDir}).Run(ctx, j, r, approved)
	}
	if wait && !now && err == nil {
		r, err = waitQueued(ctx, s.db, c, j, r)
	}
	if err != nil {
		return r, err
	}

	if (now || wait) && r.State != "succeeded" {
		return r, &Error{RunUnsuccessful, fmt.Errorf("run %s: %s", r.ID, r.State)}
	}
	return r, nil
}
func Runs(ctx context.Context, opts Options, c clock.Clock, job, state string, limit int) ([]Run, error) {
	if limit < 1 || limit > 10000 {
		return nil, &Error{Usage, fmt.Errorf("limit must be 1–10000")}
	}
	if state != "" && state != "queued" && state != "claimed" && state != "running" && !store.Terminal(state) {
		return nil, &Error{Usage, fmt.Errorf("invalid run state")}
	}
	s, err := openJobs(ctx, opts, c, true)
	if err != nil {
		return nil, err
	}
	defer s.db.Close()
	return s.db.ListRuns(ctx, job, state, limit)
}
func CancelRun(ctx context.Context, opts Options, c clock.Clock, id string) error {
	if opts.ReadOnly {
		return &Error{Usage, fmt.Errorf("now preview is read-only")}
	}
	s, err := openJobs(ctx, opts, c, false)
	if err != nil {
		return err
	}
	defer s.db.Close()
	err = runner.Cancel(ctx, s.db, s.config.DataDir, id)
	if errors.Is(err, store.ErrRunNotFound) {
		return &Error{Usage, err}
	}
	return err
}

func waitQueued(ctx context.Context, db *store.Store, c clock.Clock, j jobs.Job, r Run) (Run, error) {
	timeout, _ := time.ParseDuration(j.Timeout)
	deadline := c.After(timeout + 10*time.Second)
	for {
		current, err := db.GetRun(ctx, r.ID)
		if err != nil {
			return r, err
		}
		if store.Terminal(current.State) {
			return current, nil
		}
		select {
		case <-ctx.Done():
			return current, ctx.Err()
		case <-deadline:
			return current, fmt.Errorf("waiting for run %s timed out", r.ID)
		case <-c.After(50 * time.Millisecond):
		}
	}
}
