package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/config"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

type jobSession struct {
	config config.Config
	db     *store.Store
	now    time.Time
	views  []JobView
}

func openJobs(ctx context.Context, opts Options, c clock.Clock, dry bool) (*jobSession, error) {
	cfg, err := config.Load(opts.configOptions())
	if err != nil {
		return nil, err
	}
	for _, path := range []string{cfg.Database(), cfg.JobsDir()} {
		if _, err = os.Stat(path); os.IsNotExist(err) {
			return nil, &Error{NotInitialized, fmt.Errorf("not initialised; run agentcron init")}
		} else if err != nil {
			return nil, err
		}
	}
	dry = dry || opts.ReadOnly
	open := store.Open
	if dry {
		open = store.OpenExisting
	}
	db, err := open(ctx, cfg.Database())
	if err != nil {
		return nil, err
	}
	session := &jobSession{config: cfg, db: db, now: c.Now().UTC(), views: []JobView{}}
	if err = session.sync(ctx, !dry); err != nil {
		db.Close()
		return nil, err
	}
	return session, nil
}

// sync reconciles definitions without discarding pause, approval or interval anchors.
func (s *jobSession) sync(ctx context.Context, persist bool) error {
	entries, err := jobs.List(s.config.JobsDir())
	if err != nil {
		return err
	}
	states, err := s.db.JobStates(ctx)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	s.views = []JobView{}
	for _, entry := range entries {
		seen[entry.Name] = true
		view := JobView{Name: entry.Name, Job: entry.Job, Error: entry.Error}
		if entry.Job != nil {
			state, exists := states[entry.Name]
			if !exists {
				state = store.JobState{Name: entry.Name, Approved: entry.Job.CreatedBy == "cli", Paused: entry.Job.CreatedBy == "mcp", AnchorAt: s.now}
			}
			if state.AnchorAt.IsZero() {
				state.AnchorAt = s.now
			}
			state.Orphaned = false
			state.NextFire = nil
			fires := jobFires(*entry.Job, state.AnchorAt, s.now, 1)
			if len(fires) > 0 {
				state.NextFire = &fires[0]
			}
			state.UpdatedAt = s.now
			last, e := s.db.ListRuns(ctx, entry.Name, "", 1)
			if e != nil {
				return e
			}
			if len(last) > 0 {
				view.LastState = last[0].State
				state.LastRunID = last[0].ID
			}
			if err := s.persist(ctx, state, persist); err != nil {
				return err
			}
			view.State = &state
		}
		s.views = append(s.views, view)
	}
	for name, state := range states {
		if !seen[name] {
			state.Orphaned = true
			state.NextFire = nil
			state.UpdatedAt = s.now
			if err := s.persist(ctx, state, persist); err != nil {
				return err
			}
		}
	}
	return nil
}
