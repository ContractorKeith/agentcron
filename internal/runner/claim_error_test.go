package runner

import (
	"context"
	"github.com/ContractorKeith/agentcron/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func TestLockErrorFinalizesClaim(t *testing.T) {
	x, j := setup(t)
	ctx := context.Background()
	path := filepath.Join(x.DataDir, "runs", ".lock-"+j.Name)
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	r, err := store.NewRun(j.Name, "manual", x.Clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = x.DB.CreateRun(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err = x.DB.ClaimRun(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	r.State = "claimed"
	_, _ = x.Run(ctx, j, r, true)
	got, err := x.DB.GetRun(ctx, r.ID)
	if err != nil || got.State != "blocked_config" || got.Error == "" {
		t.Fatal(got, err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	next := executeRun(t, x, j, true)
	if next.State != "succeeded" {
		t.Fatal(next)
	}
}
