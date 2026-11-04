package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var errSetupCancelled = errors.New("setup cancelled")

func setupCancelled(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "cancel"))
	return err == nil
}

// The owner handles claimed-run cancellation; the caller never signals its PID.
func (x Runner) watchSetupCancel(ctx context.Context, dir string, cancel context.CancelCauseFunc) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if setupCancelled(dir) {
				cancel(errSetupCancelled)
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-x.Clock.After(20 * time.Millisecond):
			}
		}
	}()
	return done
}
