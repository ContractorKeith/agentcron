package daemon

import (
	"os"
	"path/filepath"
	"syscall"
)

// Wake signals only a live identity while another open file holds the lease.
func Wake(data string) bool {
	f, err := os.Open(filepath.Join(data, "daemon.lock"))
	if err != nil {
		return false
	}
	defer f.Close()
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		return false
	}
	if err != syscall.EWOULDBLOCK {
		return false
	}
	pid := Holder(data)
	return pid > 0 && syscall.Kill(pid, syscall.SIGHUP) == nil
}
