package app

import (
	"github.com/ContractorKeith/agentcron/internal/clock"
	"strings"
	"testing"
	"time"
)

func TestRefuseNeverFiringJob(t *testing.T) {
	opts := isolated(t)
	if err := Init(t.Context(), opts); err != nil {
		t.Fatal(err)
	}
	c := clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
	for _, expr := range []string{"0 0 30 2 *", "at 2026-01-01T00:00:00Z", "at 2032-01-01T00:00:00Z"} {
		j := NewJob("job")
		j.Schedule = expr
		j.Runner = "command"
		j.Command = []string{"cat"}
		j.Prompt = "hi"
		_, err := AddJob(t.Context(), opts, c, j, true)
		if Code(err) != Usage || !strings.Contains(err.Error(), "schedule never fires") {
			t.Errorf("%s: %v", expr, err)
		}
		_, err = CreateMCPJob(t.Context(), opts, c, j)
		if Code(err) != Usage || !strings.Contains(err.Error(), "schedule never fires") {
			t.Errorf("MCP %s: %v", expr, err)
		}
	}
}
