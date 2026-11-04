package runner

import (
	"context"
	"github.com/ContractorKeith/agentcron/internal/store"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExternalCancelOfCooperativeRunner(t *testing.T) {
	x, j := setup(t)
	t.Setenv("FAKE_MODE", "sleep")
	t.Setenv("FAKE_SLEEP", "30s")
	ready := filepath.Join(x.DataDir, "ready")
	t.Setenv("FAKE_RECORD", ready)
	r, err := store.NewRun(j.Name, "manual", x.Clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = x.DB.CreateRun(t.Context(), r); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	release := make(chan struct{})
	entered := make(chan struct{})
	x.beforeWait = func(<-chan error) { close(entered); <-release }
	done := make(chan store.Run, 1)
	errs := make(chan error, 1)
	go func() { got, err := x.Run(ctx, j, r, true); done <- got; errs <- err }()
	joined := false
	defer func() {
		cancel()
		select {
		case <-release:
		default:
			close(release)
		}
		if !joined {
			<-done
			<-errs
		}
	}()
	deadline := x.Clock.After(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatal("runner not ready")
		case <-x.Clock.After(time.Millisecond):
		}
	}
	select {
	case <-entered:
	case <-deadline:
		t.Fatal("owner did not start wait")
	}
	if err := cancelRun(t.Context(), x.DB, x.DataDir, r.ID, x.Clock, x.Grace); err != nil {
		t.Fatal(err)
	}
	close(release)
	got := <-done
	err = <-errs
	joined = true
	if err != nil || got.State != "cancelled" {
		t.Fatalf("state=%s error=%v", got.State, err)
	}
}
