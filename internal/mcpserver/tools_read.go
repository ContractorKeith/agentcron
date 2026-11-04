package mcpserver

import (
	"context"
	"fmt"

	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type emptyInput struct{}
type jobInput struct {
	Name string `json:"name"`
}
type runsInput struct {
	Job   string `json:"job,omitempty"`
	State string `json:"state,omitempty"`
	Limit *int   `json:"limit,omitempty"`
}
type outputInput struct {
	ID        string `json:"id"`
	Stream    string `json:"stream"`
	TailBytes *int   `json:"tail_bytes,omitempty"`
}
type firesInput struct {
	Schedule string `json:"schedule,omitempty"`
	Timezone string `json:"timezone,omitempty"`
	Count    *int   `json:"count,omitempty"`
}

func bounded(n *int, fallback, maximum int, name string) (int, error) {
	if n == nil {
		return fallback, nil
	}
	if *n < 1 || *n > maximum {
		return 0, fmt.Errorf("%s must be between 1 and %d", name, maximum)
	}
	return *n, nil
}
func addReads(s *mcp.Server, st *server) {
	register(s, st, "list_jobs", "List job definitions and state (CLI ls --json).", false, func(ctx context.Context, _ emptyInput) (any, error) {
		return app.ListJobs(ctx, st.opts, st.clock)
	})
	register(s, st, "get_job", "Show one job, state, next five fires and last five runs (CLI show --json).", false, func(ctx context.Context, in jobInput) (any, error) {
		return app.ShowJob(ctx, st.opts, st.clock, in.Name)
	})
	register(s, st, "list_runs", "List ledger runs filtered by job and state; limit defaults to 50, maximum 200.", false, func(ctx context.Context, in runsInput) (any, error) {
		n, err := bounded(in.Limit, 50, 200, "limit")
		if err != nil {
			return nil, err
		}
		if in.Job != "" {
			if _, err := app.ShowJob(ctx, st.opts, st.clock, in.Job); err != nil {
				return nil, err
			}
		}
		return app.Runs(ctx, st.opts, st.clock, in.Job, in.State, n)
	})
	register(s, st, "get_run_output", "Read stdout or stderr tail; default 16384 bytes, maximum 262144; truncated reports omitted bytes.", false, func(ctx context.Context, in outputInput) (any, error) {
		n, err := bounded(in.TailBytes, 16384, 262144, "tail_bytes")
		if err != nil {
			return nil, err
		}
		return app.TailOutput(ctx, st.opts, st.clock, in.ID, in.Stream, n)
	})
	register(s, st, "next_fires", "Preview upcoming fires (CLI next --json); count per job defaults to 5, maximum 50.", false, func(ctx context.Context, in firesInput) (any, error) {
		n, err := bounded(in.Count, 5, 50, "count")
		if err != nil {
			return nil, err
		}
		return app.NextFires(ctx, st.opts, st.clock, in.Schedule, in.Timezone, n)
	})
}
