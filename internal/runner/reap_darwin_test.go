package runner

import (
	"syscall"
	"time"
)

// launchd reaps orphan descendants; allow the signal and reap to complete.
func reapDescendant(pid int) {
	for range 100 {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
