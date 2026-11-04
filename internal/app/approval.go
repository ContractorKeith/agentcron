package app

import (
	"context"
	"fmt"
	"os"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/config"
	"github.com/ContractorKeith/agentcron/internal/daemon"
	"github.com/ContractorKeith/agentcron/internal/store"
)

// This user-facing policy message is shared verbatim by CLI and MCP.
type schedulingRefusal struct{}

func (schedulingRefusal) Error() string {
	return "Scheduling from inside a run requires allow_agent_scheduling = true in config.toml"
}

// CheckMCPWrite prevents a scheduled agent from recursively changing schedules.
func CheckMCPWrite(opts Options) error {
	return checkScheduling(opts)
}

func checkScheduling(opts Options) error {
	if err := refusePreview(opts); err != nil {
		return err
	}
	cfg, err := config.Load(opts.configOptions())
	if err != nil {
		return err
	}
	if os.Getenv("AGENTCRON_RUN_ID") != "" && !cfg.AllowAgentScheduling {
		return &Error{Refused, schedulingRefusal{}}
	}
	return nil
}

func CreateMCPJob(ctx context.Context, opts Options, c clock.Clock, j Job) (JobDetail, error) {
	if err := CheckMCPWrite(opts); err != nil {
		return JobDetail{}, err
	}
	if _, err := addJob(ctx, opts, c, j, false, "mcp"); err != nil {
		return JobDetail{}, err
	}
	return ShowJob(ctx, opts, c, j.Name)
}

// ApproveJob enables firing and resumes an agent-created job; repeated approval is harmless.
func ApproveJob(ctx context.Context, opts Options, c clock.Clock, name string) error {
	if os.Getenv("AGENTCRON_RUN_ID") != "" {
		return &Error{Refused, fmt.Errorf("approval is human-only; cannot approve inside a run")}
	}
	if err := refusePreview(opts); err != nil {
		return err
	}
	s, err := openJobs(ctx, opts, c, false)
	if err != nil {
		return err
	}
	defer s.db.Close()
	v, err := s.find(name)
	if err != nil {
		return err
	}
	st := *v.State
	if st.Approved {
		return nil
	}
	resetNextFire(&st, *v.Job, s.now)
	st.Approved, st.Paused, st.UpdatedAt = true, false, s.now
	return s.db.PutJobState(ctx, st)
}

func EnqueueMCPRun(ctx context.Context, opts Options, c clock.Clock, name string) (Run, error) {
	if err := CheckMCPWrite(opts); err != nil {
		return Run{}, err
	}
	s, err := openJobs(ctx, opts, c, false)
	if err != nil {
		return Run{}, err
	}
	defer s.db.Close()
	v, err := s.find(name)
	if err != nil {
		return Run{}, err
	}
	if !v.State.Approved {
		return Run{}, &Error{Refused, fmt.Errorf("job %q is unapproved; ask a human to run agentcron approve %s", name, name)}
	}
	if v.State.Paused {
		return Run{}, &Error{Refused, fmt.Errorf("job %q is paused; resume it before running", name)}
	}
	r, err := store.NewRun(name, "mcp", c.Now())
	if err != nil {
		return r, err
	}
	if err := s.db.CreateRun(ctx, r); err != nil {
		return r, err
	}
	daemon.Wake(s.config.DataDir)
	return r, nil
}
