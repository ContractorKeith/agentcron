package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Prefer an available exit over a simultaneous shutdown or timeout.
func (x Runner) waitProcess(ctx context.Context, pid int, done <-chan error, timer <-chan time.Time, dir string) (string, error) {
	stop := func(state string) (string, error) {
		select {
		case err := <-done:
			teardownGroup(pid, x.Clock, x.Grace)
			return "succeeded", err
		default:
		}
		err := terminate(pid, done, x.Clock, x.Grace)
		var exit *exec.ExitError
		if err == nil || (errors.As(err, &exit) && exit.ProcessState.Exited()) {
			return "succeeded", err
		}
		return state, err
	}
	for {
		select {
		case err := <-done:
			teardownGroup(pid, x.Clock, x.Grace)
			return "succeeded", err
		case <-ctx.Done():
			return stop("cancelled")
		case <-timer:
			return stop("timed_out")
		case <-x.Clock.After(50 * time.Millisecond):
			if _, err := os.Stat(filepath.Join(dir, "cancel")); err == nil {
				return stop("cancelled")
			}
		}
	}
}
