package runner

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func TestOwnerHelper(t *testing.T) {
	dir := os.Getenv("TEST_OWNER_DIR")
	if dir == "" {
		return
	}
	db, e := store.Open(context.Background(), filepath.Join(dir, "db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var j jobs.Job
	if e = json.Unmarshal([]byte(os.Getenv("TEST_OWNER_JOB")), &j); e != nil {
		t.Fatal(e)
	}
	r, e := db.GetRun(context.Background(), os.Getenv("TEST_OWNER_RUN"))
	if e != nil {
		t.Fatal(e)
	}
	_, e = (Runner{DB: db, Clock: clock.Real{}, DataDir: dir, JobsDir: dir, Grace: 200 * time.Millisecond}).Run(context.Background(), j, r, true)
	if e != nil {
		t.Fatal(e)
	}
}
func orphanRun(t *testing.T) (Runner, jobs.Job, store.Run) {
	t.Helper()
	x, j := setup(t)
	j.Timeout = "30s"
	t.Setenv("FAKE_MODE", "ignore-term")
	t.Setenv("FAKE_SLEEP", "30s")
	ready := filepath.Join(x.DataDir, "ready")
	t.Setenv("FAKE_RECORD", ready)
	r, e := store.NewRun(j.Name, "manual", x.Clock.Now())
	if e != nil {
		t.Fatal(e)
	}
	if e = x.DB.CreateRun(context.Background(), r); e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(j)
	cmd := exec.Command(os.Args[0], "-test.run=^TestOwnerHelper$")
	cmd.Env = append(os.Environ(), "TEST_OWNER_DIR="+x.DataDir, "TEST_OWNER_JOB="+string(b), "TEST_OWNER_RUN="+r.ID)
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		current, _ := x.DB.GetRun(context.Background(), r.ID)
		if current.PID != nil {
			_ = syscall.Kill(-*current.PID, syscall.SIGKILL)
			reapDescendant(*current.PID)
		}
	})
	for range 200 {
		current, err := x.DB.GetRun(context.Background(), r.ID)
		if err != nil {
			t.Fatal(err)
		}
		if current.State == "running" {
			if _, err = os.Stat(ready); err == nil {
				r = current
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if r.State != "running" {
		t.Fatal("owner did not launch")
	}
	if e = cmd.Process.Kill(); e != nil {
		t.Fatal(e)
	}
	_ = cmd.Wait()
	return x, j, r
}
func TestCancelAfterOwnerDies(t *testing.T) {
	x, _, r := orphanRun(t)
	if e := Cancel(context.Background(), x.DB, x.DataDir, r.ID); e != nil {
		t.Fatal(e)
	}
	reapDescendant(*r.PID)
	got, e := x.DB.GetRun(context.Background(), r.ID)
	if e != nil || got.State != "cancelled" || processLive(*r.PID) {
		t.Fatalf("state=%s child live=%v err=%v", got.State, processLive(*r.PID), e)
	}
}
func TestOverlapAfterOwnerDies(t *testing.T) {
	x, j, _ := orphanRun(t)
	t.Setenv("FAKE_MODE", "ok")
	r := executeRun(t, x, j, true)
	if r.State != "skipped" {
		t.Fatalf("overlapping run %s", r.State)
	}
}
func TestLaunchTimestampUsesClock(t *testing.T) {
	x, j := setup(t)
	r := executeRun(t, x, j, true)
	if r.StartedAt == nil || r.StartedAt.Before(r.ScheduledFor) {
		t.Fatalf("started=%v scheduled=%v", r.StartedAt, r.ScheduledFor)
	}
}
func TestStartFailureFinalizes(t *testing.T) {
	x, j := setup(t)
	e := x.DB.WithImmediateTx(context.Background(), func(tx *store.ImmediateTx) error {
		_, e := tx.Exec(context.Background(), `CREATE TRIGGER reject_start BEFORE UPDATE OF state ON runs WHEN NEW.state='running' BEGIN SELECT RAISE(FAIL,'start failure'); END`)
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	r, e := store.NewRun(j.Name, "manual", x.Clock.Now())
	if e != nil {
		t.Fatal(e)
	}
	if e = x.DB.CreateRun(context.Background(), r); e != nil {
		t.Fatal(e)
	}
	_, _ = x.Run(context.Background(), j, r, true)
	got, e := x.DB.GetRun(context.Background(), r.ID)
	if e != nil || got.State != "failed" || got.Error == "" {
		t.Fatalf("%+v %v", got, e)
	}
	if _, e = os.Stat(got.Worktree); !os.IsNotExist(e) {
		t.Fatal("failed start left worktree")
	}
}
