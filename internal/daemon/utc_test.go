package daemon

import (
	"bytes"
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
	"strings"
	"testing"
	"time"
)

func TestSkippedRunLogUTC(t *testing.T) {
	e, ctx := fixture(t)
	e.Clock = clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.FixedZone("EST", -5*3600)))
	var out bytes.Buffer
	e.Log = &out
	r, err := store.NewRun("job", "manual", e.Clock.Now())
	check(t, err)
	check(t, e.DB.CreateRun(ctx, r))
	_, err = e.Claim(ctx, map[string]jobs.Job{"job": {}}, map[string]store.JobState{"job": {Paused: true}})
	check(t, err)
	if !strings.Contains(out.String(), `"ended_at":"2026-11-04T17:00:00Z"`) {
		t.Fatal(out.String())
	}
	got, err := e.DB.GetRun(ctx, r.ID)
	check(t, err)
	if !strings.HasSuffix(got.EndedAt.Format(time.RFC3339Nano), "Z") {
		t.Fatal(got.EndedAt)
	}
}
