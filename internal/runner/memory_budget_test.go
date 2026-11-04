package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/store"
)

func TestMemoryBootstrapHonorsJobTimeoutWithoutLaunchingAgent(t *testing.T) {
	x, j := setup(t)
	j.Memory = true
	j.Timeout = "150ms"
	t.Setenv("FAKE_MDKB_CAT_SLEEP", "10s")
	record := filepath.Join(x.DataDir, "record")
	t.Setenv("FAKE_RECORD", record)
	start := x.Clock.Now()
	r := executeRun(t, x, j, true)
	if r.State != "timed_out" || !strings.Contains(r.Error, "setup timed out") || x.Clock.Now().Sub(start) > 2*time.Second {
		t.Fatalf("%+v", r)
	}
	for _, entry := range records(t, record) {
		if filepath.Base(entry.Argv[0]) != "mdkb" {
			t.Fatal("agent launched during expired setup")
		}
	}
}

func TestCancelDuringMemoryBootstrapNeverLaunchesAgent(t *testing.T) {
	x, j := setup(t)
	j.Memory = true
	j.Timeout = "20s"
	t.Setenv("FAKE_MDKB_CAT_SLEEP", "10s")
	record := filepath.Join(x.DataDir, "record")
	t.Setenv("FAKE_RECORD", record)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, err := store.NewRun(j.Name, "manual", x.Clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = x.DB.CreateRun(ctx, r); err != nil {
		t.Fatal(err)
	}
	type result struct {
		run store.Run
		err error
	}
	done := make(chan result, 1)
	go func() { r, err := x.Run(ctx, j, r, true); done <- result{r, err} }()
	// Always join the owner, even when a liveness assertion fails.
	joined := false
	defer func() {
		cancel()
		if !joined {
			<-done
		}
	}()
	deadline := x.Clock.After(5 * time.Second)
	for {
		if data, _ := os.ReadFile(record); len(data) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("bootstrap did not start")
		case <-x.Clock.After(10 * time.Millisecond):
		}
	}
	if err = Cancel(context.Background(), x.DB, x.DataDir, r.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		joined = true
		if got.err != nil || got.run.State != "cancelled" {
			t.Fatalf("%+v", got)
		}
	case <-x.Clock.After(2 * time.Second):
		t.Fatal("cancel did not interrupt bootstrap")
	}
	for _, entry := range records(t, record) {
		if filepath.Base(entry.Argv[0]) != "mdkb" {
			t.Fatal("agent launched after setup cancel")
		}
	}
}

func TestMemoryNewHasLongerCapWithinJobBudget(t *testing.T) {
	x, j := setup(t)
	j.Memory = true
	j.Timeout = "10s"
	root := os.Getenv("MDKB_ROOT")
	if err := os.MkdirAll(filepath.Join(root, ".mdkb"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_MDKB_NEW_SLEEP", "5200ms")
	r := executeRun(t, x, j, true)
	if r.State != "succeeded" || r.Error != "" {
		t.Fatalf("%+v", r)
	}
	if _, err := os.Stat(filepath.Join(root, "agentcron", j.Name+".md")); err != nil {
		t.Fatal(err)
	}
}
