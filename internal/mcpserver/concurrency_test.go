package mcpserver

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSlowReadDoesNotBlockOtherTools(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	st := &server{}
	started, release := make(chan struct{}), make(chan struct{})
	register(s, st, "get_run_output", "Read output", false, func(context.Context, emptyInput) (any, error) {
		close(started)
		<-release
		return "output", nil
	})
	register(s, st, "list_jobs", "List jobs", false, func(context.Context, emptyInput) (any, error) { return []string{}, nil })
	a, b := mcp.NewInMemoryTransports()
	ss, err := s.Connect(t.Context(), a, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(t.Context(), b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	done := make(chan error, 1)
	go func() {
		_, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "get_run_output", Arguments: map[string]any{}})
		done <- err
	}()
	<-started
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_jobs", Arguments: map[string]any{}})
	close(release)
	if firstErr := <-done; firstErr != nil {
		t.Fatal(firstErr)
	}
	if err != nil || result.IsError {
		t.Fatalf("slow output blocked another read: %v %+v", err, result)
	}
}
