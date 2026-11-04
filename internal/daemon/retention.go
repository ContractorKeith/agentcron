package daemon

import (
	"context"
	"os"
	"path/filepath"

	"github.com/ContractorKeith/agentcron/internal/store"
)

func (e *Engine) Retention(ctx context.Context) error {
	runs, err := e.DB.ListRuns(ctx, "", "", -1)
	if err != nil {
		return err
	}
	counts := map[string]int{}
	for _, r := range runs {
		if !store.Terminal(r.State) {
			continue
		}
		counts[r.Job]++
		if counts[r.Job] <= e.KeepRuns || r.Kept {
			continue
		}
		if err := os.RemoveAll(filepath.Join(e.DataDir, "runs", r.ID)); err != nil {
			return err
		}
		if err := e.DB.DeleteTerminal(ctx, r.ID); err != nil {
			return err
		}
		e.log(map[string]string{"run": r.ID, "state": "pruned"})
	}
	return nil
}
