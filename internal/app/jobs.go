package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/schedule"
	"github.com/ContractorKeith/agentcron/internal/store"
)

type Job = jobs.Job

func NewJob(name string) Job { return jobs.Defaults(name) }

type JobView struct {
	Status    string          `json:"status"`
	Name      string          `json:"name"`
	Job       *Job            `json:"job,omitempty"`
	State     *store.JobState `json:"state,omitempty"`
	Error     string          `json:"error,omitempty"`
	LastState string          `json:"last_state,omitempty"`
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
			if view.Job == nil {
				break
			}
			return view, nil
		}
	}
	return JobView{}, &Error{Usage, fmt.Errorf("unknown job %q", name)}
}

type JobDetail struct {
	Status string         `json:"status"`
	Error  string         `json:"error,omitempty"`
	Job    Job            `json:"job"`
	State  store.JobState `json:"state"`
	Next   []time.Time    `json:"next"`
	Runs   []store.Run    `json:"runs"`
}

func ShowJob(ctx context.Context, opts Options, c clock.Clock, name string) (JobDetail, error) {
	s, err := openJobs(ctx, opts, c, false)
	if err != nil {
		return JobDetail{}, err
	}
	defer s.db.Close()
	view, err := s.find(name)
	if err != nil {
		for _, candidate := range s.views {
			if candidate.Name == name && (candidate.Status == "orphaned" || candidate.Status == "invalid") {
				view = candidate
				err = nil
				break
			}
		}
		if err != nil {
			return JobDetail{}, err
		}
	}
	runs, err := s.db.ListRuns(ctx, name, "", 5)
	if err != nil {
		return JobDetail{}, err
	}
	if view.Job == nil {
		return JobDetail{Status: view.Status, Error: view.Error, Job: Job{Name: name}, State: *view.State, Next: []time.Time{}, Runs: runs}, nil
	}
	return JobDetail{view.Status, "", *view.Job, *view.State, jobFires(*view.Job, view.State.AnchorAt, s.now, 5), runs}, nil
}

func (s *jobSession) persist(ctx context.Context, state store.JobState, enabled bool) error {
	if !enabled {
		return nil
	}
	return s.db.SyncDefinition(ctx, state)
}

func jobStatus(j Job, st store.JobState, runs []store.Run) string {
	switch {
	case st.Orphaned:
		return "orphaned"
	case !st.Approved:
		return "unapproved"
	case st.Paused:
		return "paused"
	}
	if st.NextFire != nil {
		return "active"
	}
	if strings.HasPrefix(j.Schedule, "at ") {
		if st.Fired {
			return "done"
		}
		for _, r := range runs {
			if r.Trigger == "schedule" || r.Trigger == "catchup" {
				return "done"
			}
		}
	}
	return "never"
}
