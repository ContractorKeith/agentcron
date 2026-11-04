// Package mcpserver exposes app use cases over MCP stdio.
package mcpserver

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type server struct {
	opts  app.Options
	clock clock.Clock
	mu    sync.Mutex
}

func New(ctx context.Context, opts app.Options, c clock.Clock, allowWrite bool, version string) (*mcp.Server, error) {
	if _, err := app.ListJobs(ctx, opts, c); err != nil {
		return nil, err
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "agentcron", Version: version}, nil)
	state := &server{opts: opts, clock: c}
	addReads(s, state)
	if allowWrite {
		addWrites(s, state)
	}
	return s, nil
}

func Run(ctx context.Context, opts app.Options, allowWrite bool, version string) error {
	s, err := New(ctx, opts, clock.Real{}, allowWrite, version)
	if err != nil {
		return err
	}
	return s.Run(ctx, &mcp.StdioTransport{})
}

// Explicit structured content preserves CLI arrays rather than wrapping them in objects.
func register[In any](s *mcp.Server, state *server, name, description string, write bool, fn func(context.Context, In) (any, error)) {
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: description}, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		state.mu.Lock()
		defer state.mu.Unlock()
		if write {
			if err := app.CheckMCPWrite(state.opts); err != nil {
				return nil, nil, err
			}
		}
		out, err := fn(ctx, in)
		if err != nil {
			return nil, nil, err
		}
		raw, err := json.Marshal(out)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}, StructuredContent: out}, nil, nil
	})
}
