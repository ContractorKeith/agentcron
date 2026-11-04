package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func AddJob(ctx context.Context, opts Options, c clock.Clock, j Job, dry bool) (string, error) {
	return addJob(ctx, opts, c, j, dry, "cli")
}

func addJob(ctx context.Context, opts Options, c clock.Clock, j Job, dry bool, source string) (string, error) {
	if err := checkScheduling(opts); err != nil {
		return "", err
	}
	s, err := openJobs(ctx, opts, c, dry)
	if err != nil {
		return "", err
	}
	defer s.db.Close()
	exists, err := jobs.Exists(s.config.JobsDir(), j.Name)
	if err != nil {
		return "", &Error{Usage, err}
	}
	if exists {
		return "", &Error{Usage, fmt.Errorf("job %q already exists", j.Name)}
	}
	// Creation provenance belongs to the interface, never to caller-supplied TOML.
	j.CreatedBy = source
	if err := j.Validate(s.config.JobsDir()); err != nil {
		return "", &Error{Usage, err}
	}
	text, err := jobs.Encode(j)
	if err != nil {
		return "", err
	}
	if dry {
		return text, nil
	}
	if err := jobs.Create(s.config.JobsDir(), j); err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", &Error{Usage, fmt.Errorf("job %q already exists", j.Name)}
		}
		return "", err
	}
	if err := s.db.PutJobState(ctx, store.JobState{
		Name: j.Name, Approved: source == "cli", Paused: source == "mcp", AnchorAt: s.now, UpdatedAt: s.now,
	}); err != nil {
		return "", err
	}
	if err := s.sync(ctx, true); err != nil {
		return "", err
	}
	return text, nil
}

func RemoveJob(ctx context.Context, opts Options, c clock.Clock, name string, dry bool) error {
	if err := checkScheduling(opts); err != nil {
		return err
	}
	s, err := openJobs(ctx, opts, c, dry)
	if err != nil {
		return err
	}
	defer s.db.Close()
	// Invalid definitions must still be removable.
	exists, err := jobs.Exists(s.config.JobsDir(), name)
	if err != nil {
		return &Error{Usage, err}
	}
	if !exists {
		return &Error{Usage, fmt.Errorf("unknown job %q", name)}
	}
	if dry {
		return nil
	}
	if err := jobs.Remove(s.config.JobsDir(), name); err != nil {
		return err
	}
	return s.sync(ctx, true)
}

func PauseJob(ctx context.Context, opts Options, c clock.Clock, name string, paused bool) error {
	if err := checkScheduling(opts); err != nil {
		return err
	}
	s, err := openJobs(ctx, opts, c, false)
	if err != nil {
		return err
	}
	defer s.db.Close()
	view, err := s.find(name)
	if err != nil {
		return err
	}
	state := *view.State
	state.Paused = paused
	return s.db.PutJobState(ctx, state)
}

func refusePreview(opts Options) error {
	if opts.ReadOnly {
		return &Error{Usage, fmt.Errorf("--now is a read-only preview; write commands are refused")}
	}
	return nil
}
