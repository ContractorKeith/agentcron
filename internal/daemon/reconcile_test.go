package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/store"
)

func TestRestartRules(t *testing.T) {
	for _, kind := range []string{"dead", "recycled", "orphan", "claimed", "live"} {
		t.Run(kind, func(t *testing.T) {
			e, ctx := fixture(t)
			now := e.Clock.Now()
			r, err := store.NewRun("job", "manual", now)
			check(t, err)
			check(t, e.DB.CreateRun(ctx, r))
			check(t, e.DB.ClaimOwned(ctx, r.ID, now, now))
			pid, owner := 123, 456
			r.PID = &pid
			r.OwnerPID = &owner
			r.OwnerStart = &now
			r.StartedAt = &now
			r.ProcStart = &now
			if kind != "claimed" {
				check(t, e.DB.StartOwnedRun(ctx, r))
			}
			kills := 0
			p := Processes{Alive: func(pid int) bool { return kind != "dead" && (kind != "orphan" || pid != owner) && kind != "claimed" }, Start: func(int) (time.Time, error) {
				if kind == "recycled" {
					return now.Add(time.Second), nil
				}
				return now, nil
			}, Kill: func(int) error { kills++; return nil }}
			check(t, e.reconcile(ctx, p))
			got, err := e.DB.GetRun(ctx, r.ID)
			check(t, err)
			want := "unknown"
			if kind == "live" {
				want = "running"
			}
			if got.State != want {
				t.Fatal(got)
			}
			if (kind == "orphan") != (kills == 1) {
				t.Fatal("unsafe kill", kind, kills)
			}
			check(t, e.reconcile(ctx, p))
			if kind == "orphan" && kills != 1 {
				t.Fatal("replayed unknown")
			}
		})
	}
}
func TestRetentionKeepsLiveAndKept(t *testing.T) {
	e, ctx := fixture(t)
	for i := 0; i < 6; i++ {
		r, err := store.NewRun("job", "manual", e.Clock.Now())
		check(t, err)
		check(t, e.DB.CreateRun(ctx, r))
		path := filepath.Join(e.DataDir, "runs", r.ID)
		check(t, os.MkdirAll(path, 0700))
		if i == 5 {
			continue
		}
		check(t, e.DB.ClaimRun(ctx, r.ID))
		pid := 123
		at := e.Clock.Now()
		check(t, e.DB.StartRun(ctx, r.ID, pid, at, ""))
		r.State = "succeeded"
		r.EndedAt = &at
		r.Kept = i == 0
		check(t, e.DB.FinishRun(ctx, r))
	}
	check(t, e.Retention(ctx))
	runs, _ := e.DB.ListRuns(ctx, "job", "", -1)
	if len(runs) != 4 {
		t.Fatal(fmt.Sprint(runs))
	}
	for _, r := range runs {
		if _, err := os.Stat(filepath.Join(e.DataDir, "runs", r.ID)); err != nil {
			t.Fatal(err)
		}
	}
}
