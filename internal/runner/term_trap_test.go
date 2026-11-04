package runner

import (
	"context"
	"github.com/ContractorKeith/agentcron/internal/store"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTermTrapKeepsRequestedOutcome(t *testing.T) {
	for _, want := range []string{"timed_out", "cancelled"} {
		t.Run(want, func(t *testing.T) {
			x, j := setup(t)
			ready := filepath.Join(x.DataDir, "trap-ready")
			j.Runner = "command"
			j.Command = []string{"sh", "-c", `trap 'exit 0' TERM; echo ready > "$1"; while :; do :; done`, "trap", ready}
			j.Timeout = "1s"
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			x.Observe = func(store.Run) {
				deadline := x.Clock.After(5 * time.Second)
				for {
					if _, err := os.Stat(ready); err == nil {
						break
					}
					select {
					case <-deadline:
						t.Error("trap not ready")
						cancel()
						return
					case <-x.Clock.After(time.Millisecond):
					}
				}
				if want == "cancelled" {
					cancel()
				}
			}
			r, err := store.NewRun(j.Name, "manual", x.Clock.Now())
			if err != nil {
				t.Fatal(err)
			}
			if err = x.DB.CreateRun(ctx, r); err != nil {
				t.Fatal(err)
			}
			got, err := x.Run(ctx, j, r, true)
			if err != nil || got.State != want || got.ExitCode == nil || *got.ExitCode != 0 {
				t.Fatalf("state=%s exit=%v err=%v", got.State, got.ExitCode, err)
			}
		})
	}
}
