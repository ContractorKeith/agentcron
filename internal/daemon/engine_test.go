package daemon

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/runner"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func fixture(t *testing.T) (*Engine, context.Context) {
	t.Helper()
	ctx := context.Background()
	data := t.TempDir()
	for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(key, data)
	}
	t.Setenv("MDKB_ROOT", filepath.Join(data, "kb"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(data, "gitconfig"))
	db, err := store.Open(ctx, filepath.Join(data, "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	c := clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
	return &Engine{DB: db, Clock: c, DataDir: data, Concurrency: 2, KeepRuns: 2}, ctx
}
func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func TestLockNamesHolder(t *testing.T) {
	data := t.TempDir()
	f, err := Lock(data)
	check(t, err)
	defer f.Close()
	_, err = Lock(data)
	locked, ok := err.(*LockedError)
	if !ok || locked.PID == "" || Holder(data) == 0 {
		t.Fatal(err)
	}
	f.Close()
	f, err = Lock(data)
	check(t, err)
	f.Close()
	if Holder(data) != 0 {
		t.Fatal("lock still held")
	}
}
func TestCatchupAndDedupe(t *testing.T) {
	for _, policy := range []string{"one", "none"} {
		t.Run(policy, func(t *testing.T) {
			e, ctx := fixture(t)
			now := e.Clock.Now()
			first := now.Add(-3 * time.Minute)
			st := store.JobState{Name: "job", Approved: true, AnchorAt: first.Add(-time.Minute), NextFire: &first, UpdatedAt: now}
			j := jobs.Defaults("job")
			j.Schedule = "every 1m"
			j.Catchup = policy
			defs := map[string]jobs.Job{"job": j}
			states := map[string]store.JobState{"job": st}
			check(t, e.DB.PutJobState(ctx, st))
			check(t, e.Enqueue(ctx, defs, states))
			check(t, e.Enqueue(ctx, defs, states))
			runs, err := e.DB.ListRuns(ctx, "job", "", -1)
			check(t, err)
			want := 1
			if policy == "one" {
				want = 2
			}
			if len(runs) != want {
				t.Fatalf("%+v", runs)
			}
			if policy == "one" && (runs[1].Trigger != "catchup" || !runs[1].ScheduledFor.Equal(now.Add(-time.Minute))) {
				t.Fatal(runs)
			}
			saved, err := e.DB.JobStates(ctx)
			check(t, err)
			if !saved["job"].NextFire.Equal(now.Add(time.Minute)) {
				t.Fatal(saved)
			}
			// A backwards wall clock and a fresh Engine cannot replay an earlier cursor.
			earlier := clock.NewFake(now.Add(-time.Hour))
			e.Clock = earlier
			check(t, e.Enqueue(ctx, defs, saved))
			after, _ := e.DB.ListRuns(ctx, "job", "", -1)
			if len(after) != want {
				t.Fatal(after)
			}
		})
	}
}
func TestOneShotAndWindow(t *testing.T) {
	for _, age := range []time.Duration{0, 10 * time.Second, 30 * time.Second, time.Hour} {
		e, ctx := fixture(t)
		fire := e.Clock.Now().Add(-age)
		j := jobs.Defaults("once")
		j.Schedule = "at " + fire.Format(time.RFC3339)
		j.Catchup = "none"
		st := store.JobState{Name: j.Name, Approved: true, NextFire: &fire, UpdatedAt: e.Clock.Now()}
		check(t, e.DB.PutJobState(ctx, st))
		check(t, e.Enqueue(ctx, map[string]jobs.Job{j.Name: j}, map[string]store.JobState{j.Name: st}))
		runs, _ := e.DB.ListRuns(ctx, j.Name, "", -1)
		want := 0
		if age < 30*time.Second {
			want = 1
		}
		if len(runs) != want {
			t.Fatal(age, runs)
		}
		states, _ := e.DB.JobStates(ctx)
		if states[j.Name].NextFire != nil {
			t.Fatal(states)
		}
	}
}
func TestClaimOldestConcurrencyAndOverlap(t *testing.T) {
	e, ctx := fixture(t)
	defs := map[string]jobs.Job{}
	states := map[string]store.JobState{}
	for _, name := range []string{"a", "a", "b", "c", "paused"} {
		j := jobs.Defaults(name)
		defs[name] = j
		states[name] = store.JobState{Name: name, Approved: true, Paused: name == "paused"}
		r, err := store.NewRun(name, "manual", e.Clock.Now())
		check(t, err)
		check(t, e.DB.CreateRun(ctx, r))
	}
	claimed, err := e.Claim(ctx, defs, states)
	check(t, err)
	if len(claimed) != 2 || claimed[0].Job != "a" || claimed[1].Job != "b" {
		t.Fatal(claimed)
	}
	skipped, _ := e.DB.ListRuns(ctx, "", "skipped", -1)
	if len(skipped) != 2 {
		t.Fatal(skipped)
	}
	queued, _ := e.DB.ListRuns(ctx, "", "queued", -1)
	if len(queued) != 1 || queued[0].Job != "c" {
		t.Fatal(queued)
	}
}

func TestStartClaimedRun(t *testing.T) {
	e, ctx := fixture(t)
	j := jobs.Defaults("command")
	j.Memory = false // This test exercises queue ownership without a KB dependency.
	j.Schedule = "@daily"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hello"
	r, err := store.NewRun(j.Name, "manual", e.Clock.Now())
	check(t, err)
	check(t, e.DB.CreateRun(ctx, r))
	check(t, e.DB.ClaimOwned(ctx, r.ID, e.Clock.Now(), e.Clock.Now()))
	r.State = "claimed"
	observed := make(chan store.Run, 1)
	e.Runner = runner.Runner{DB: e.DB, Clock: e.Clock, DataDir: e.DataDir, JobsDir: t.TempDir(), Observe: func(r store.Run) { observed <- r }}
	e.Start(ctx, []store.Run{r}, map[string]jobs.Job{j.Name: j})
	if got := <-observed; got.State != "running" || got.PID == nil {
		t.Fatal(got)
	}
	e.Wait()
	got, err := e.DB.GetRun(ctx, r.ID)
	check(t, err)
	if got.State != "succeeded" {
		t.Fatal(got)
	}
}
