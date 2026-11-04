package daemon

import (
	"github.com/ContractorKeith/agentcron/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func TestRetentionDeleteFailurePreservesRunDirectory(t *testing.T) {
	e, ctx := fixture(t)
	e.KeepRuns = 1
	var oldDir string
	for range 2 {
		r, err := store.NewRun("job", "manual", e.Clock.Now())
		check(t, err)
		check(t, e.DB.CreateRun(ctx, r))
		dir := filepath.Join(e.DataDir, "runs", r.ID)
		check(t, os.MkdirAll(dir, 0700))
		if oldDir == "" {
			oldDir = dir
		}
		at := e.Clock.Now()
		r.State = "skipped"
		r.EndedAt = &at
		check(t, e.DB.FinishRun(ctx, r))
	}
	check(t, e.DB.WithImmediateTx(ctx, func(tx *store.ImmediateTx) error {
		_, err := tx.Exec(ctx, "CREATE TRIGGER refuse_delete BEFORE DELETE ON runs BEGIN SELECT RAISE(FAIL,'delete refused'); END")
		return err
	}))
	if err := e.Retention(ctx); err == nil {
		t.Fatal("expected ledger deletion failure")
	}
	if _, err := os.Stat(oldDir); err != nil {
		t.Fatalf("deleted directory before ledger commit: %v", err)
	}
}
