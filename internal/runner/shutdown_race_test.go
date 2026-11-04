package runner

import (
	"context"
	"github.com/ContractorKeith/agentcron/internal/store"
	"testing"
	"time"
)

func TestCompletedRunWinsShutdown(t *testing.T) {
	x, j := setup(t)
	for range 20 {
		ctx, cancel := context.WithCancel(t.Context())
		x.beforeWait = func(done <-chan error) {
			deadline := x.Clock.After(5 * time.Second)
			for len(done) == 0 {
				select {
				case <-deadline:
					t.Fatal("runner did not finish")
				case <-x.Clock.After(time.Millisecond):
				}
			}
			cancel()
		}
		r, err := store.NewRun(j.Name, "manual", x.Clock.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err = x.DB.CreateRun(ctx, r); err != nil {
			t.Fatal(err)
		}
		got, err := x.Run(ctx, j, r, true)
		cancel()
		if err != nil || got.State != "succeeded" {
			t.Fatalf("state=%s error=%v", got.State, err)
		}
	}
}
