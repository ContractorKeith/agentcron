package mcpserver

import (
	"context"
	"fmt"

	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type createInput struct {
	Catchup    *string  `json:"catchup,omitempty"`
	Name       string   `json:"name"`
	Schedule   string   `json:"schedule"`
	Runner     string   `json:"runner"`
	Prompt     string   `json:"prompt,omitempty"`
	PromptFile string   `json:"prompt_file,omitempty"`
	Repo       string   `json:"repo,omitempty"`
	Base       string   `json:"base,omitempty"`
	Timezone   string   `json:"timezone,omitempty"`
	Args       []string `json:"args,omitempty"`
	Command    []string `json:"command,omitempty"`
	Timeout    *string  `json:"timeout,omitempty"`
	Expect     string   `json:"expect,omitempty"`
	Memory     *bool    `json:"memory,omitempty"`
	NoMemory   *bool    `json:"no_memory,omitempty"`
}

func addWrites(s *mcp.Server, st *server) {
	register(s, st, "create_job", "Create a paused, unapproved job. A human must run agentcron approve before it fires.", true, func(ctx context.Context, in createInput) (any, error) {
		j := app.NewJob(in.Name)
		j.Schedule, j.Runner, j.Prompt, j.PromptFile = in.Schedule, in.Runner, in.Prompt, in.PromptFile
		j.Repo, j.Base, j.Timezone = in.Repo, in.Base, in.Timezone
		if in.Args != nil {
			j.Args = in.Args
		}
		j.Command = in.Command
		if in.Catchup != nil {
			j.Catchup = *in.Catchup
		}
		if in.Timeout != nil {
			j.Timeout = *in.Timeout
		}
		j.Expect = in.Expect
		if in.Memory != nil && in.NoMemory != nil {
			return nil, fmt.Errorf("memory and no_memory are mutually exclusive")
		}
		switch {
		case in.Memory != nil:
			j.Memory = *in.Memory
		case in.NoMemory != nil:
			j.Memory = !*in.NoMemory
		default:
			var err error
			j.Memory, err = app.DefaultMemory(st.opts)
			if err != nil {
				return nil, err
			}
		}
		return app.CreateMCPJob(ctx, st.opts, st.clock, j)
	})
	for _, paused := range []bool{true, false} {
		name := "resume_job"
		description := "Resume a paused job; this does not approve an unapproved job."
		if paused {
			name = "pause_job"
			description = "Pause a job to prevent scheduled or queued runs from starting."
		}
		register(s, st, name, description, true, func(ctx context.Context, in jobInput) (any, error) {
			if err := app.PauseJob(ctx, st.opts, st.clock, in.Name, paused); err != nil {
				return nil, err
			}
			return app.ShowJob(ctx, st.opts, st.clock, in.Name)
		})
	}
	register(s, st, "run_job", "Enqueue a run with trigger=mcp. Unapproved jobs are refused; daemon or tick executes the queue.", true, func(ctx context.Context, in jobInput) (any, error) {
		return app.EnqueueMCPRun(ctx, st.opts, st.clock, in.Name)
	})
}
