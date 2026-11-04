package daemon

import (
	"bytes"
	"context"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTickRejectsDeletedOrReplacedLedger(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "deleted", true: "replaced"}[replace], func(t *testing.T) {
			e, ctx := fixture(t)
			var log bytes.Buffer
			e.Log = &log
			synced := false
			e.Sync = func(context.Context) (map[string]jobs.Job, error) { synced = true; return map[string]jobs.Job{}, nil }
			path := filepath.Join(e.DataDir, "db")
			check(t, os.Rename(path, path+".old"))
			if replace {
				check(t, os.WriteFile(path, []byte{}, 0600))
			}
			if err := e.Tick(ctx); err == nil {
				t.Fatal("tick accepted missing/replaced ledger")
			}
			if synced {
				t.Error("continued scheduling after ledger disappeared")
			}
			if !strings.Contains(log.String(), "ledger") {
				t.Fatal("missing diagnostic", log.String())
			}
		})
	}
}
