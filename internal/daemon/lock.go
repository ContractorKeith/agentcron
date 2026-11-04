// Package daemon owns scheduling passes and the process lifetime lock.
package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type LockedError struct{ PID string }

func (e *LockedError) Error() string { return "daemon lock held by pid " + e.PID }
func Lock(data string) (*os.File, error) {
	f, err := os.OpenFile(filepath.Join(data, "daemon.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		b, _ := os.ReadFile(f.Name())
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, &LockedError{strings.TrimSpace(string(b))}
		}
		return nil, err
	}
	if err = f.Truncate(0); err == nil {
		_, err = fmt.Fprintln(f, os.Getpid())
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// Holder inspects an existing lock without creating a file or changing its pid.
func Holder(data string) int {
	f, e := os.OpenFile(filepath.Join(data, "daemon.lock"), os.O_RDWR, 0)
	if e != nil {
		return 0
	}
	defer f.Close()
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil {
		return 0
	}
	b, _ := os.ReadFile(f.Name())
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return pid
}
