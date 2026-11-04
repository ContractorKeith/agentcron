package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/config"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/schedule"
	"github.com/ContractorKeith/agentcron/internal/store"
)

type Job = jobs.Job

func NewJob(name string) Job { return jobs.Defaults(name) }

type JobView struct {
	Name  string          `json:"name"`
	Job   *Job            `json:"job,omitempty"`
	State *store.JobState `json:"state,omitempty"`
	Error string          `json:"error,omitempty"`
}

type jobSession struct {
	config config.Config
	db     *store.Store
	now    time.Time
	views  []JobView
}

func openJobs(ctx context.Context, opts Options, c clock.Clock, dry bool) (*jobSession, error) {
	cfg, err := config.Load(opts)
	if err != nil {
		return nil, err
	}
	if _, err = os.Stat(cfg.Database()); os.IsNotExist(err) {
		return nil, &Error{NotInitialized, fmt.Errorf("not initialised; run agentcron init")}
	} else if err != nil {
		return nil, err
	}
	open := store.Open
	if dry {
		open = store.OpenExisting
	}
	db, err := open(ctx, cfg.Database())
	if err != nil {
		return nil, err
	}
	session := &jobSession{config: cfg, db: db, now: c.Now(), views: []JobView{}}
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

func jobFires(j Job, anchor, after time.Time, count int) []time.Time {
	loc := time.Local
	if j.Timezone != "" {
		loc, _ = time.LoadLocation(j.Timezone)
	}
	parsed, _ := schedule.Parse(j.Schedule)
	return fires(parsed.WithAnchor(anchor), after, loc, count)
}
func fires(s schedule.Schedule, after time.Time, loc *time.Location, count int) []time.Time {
	result := []time.Time{}
	for range count {
		next, ok := s.Next(after, loc)
		if !ok {
			break
		}
		result = append(result, next.UTC())
		after = next
	}
	return result
}

func ListJobs(ctx context.Context, opts Options, c clock.Clock) ([]JobView, error) {
	s, err := openJobs(ctx, opts, c, false)
	if err != nil {
		return nil, err
	}
	defer s.db.Close()
	return s.views, nil
}

func (s *jobSession) find(name string) (JobView, error) {
	for _, view := range s.views {
		if view.Name == name {
			if view.Error != "" {
				return view, &Error{Usage, fmt.Errorf("%s: %s", name, view.Error)}
			}
			return view, nil
		}
	}
	return JobView{}, &Error{Usage, fmt.Errorf("unknown job %q", name)}
}

type JobDetail struct {
	Job   Job            `json:"job"`
	State store.JobState `json:"state"`
	Next  []time.Time    `json:"next"`
	Runs  []any          `json:"runs"`
}

func ShowJob(ctx context.Context, opts Options, c clock.Clock, name string) (JobDetail, error) {
	s, err := openJobs(ctx, opts, c, false)
	if err != nil {
		return JobDetail{}, err
	}
	defer s.db.Close()
	view, err := s.find(name)
	if err != nil {
		return JobDetail{}, err
	}
	return JobDetail{*view.Job, *view.State, jobFires(*view.Job, view.State.AnchorAt, s.now, 5), []any{}}, nil
}

func (s *jobSession) persist(ctx context.Context, state store.JobState, enabled bool) error {
	if !enabled {
		return nil
	}
	return s.db.PutJobState(ctx, state)
}
