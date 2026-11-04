package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/runner"
	"github.com/ContractorKeith/agentcron/internal/store"
)

type Run = store.Run

func RunJob(ctx context.Context, opts Options, c clock.Clock, name string, now, wait bool) (Run, error) {
	if wait && !now {
		return Run{}, &Error{Usage, fmt.Errorf("the daemon executes queued runs; use --now to run inline")}
	}
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
