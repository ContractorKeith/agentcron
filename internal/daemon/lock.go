// Package daemon owns scheduling passes and the process lifetime lock.
package daemon

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ContractorKeith/agentcron/internal/runner"
)

type LockedError struct{ PID string }

func (e *LockedError) Error() string { return "daemon lock held by pid " + e.PID }

type lockRecord struct {
	PID     int
	Start   time.Time
	Command string
}
type Lease struct {
	*os.File
	transferred bool
}

func (l *Lease) Transfer() { l.transferred = true }
func (l *Lease) Close() error {
	if !l.transferred {
		b, _ := os.ReadFile(l.Name())
		r, err := readLock(b)
		if err == nil && r.PID == os.Getpid() {
			_ = l.Truncate(0)
		}
	}
	return l.File.Close()
}
func processCommand(pid int) (string, error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	return strings.TrimSpace(string(out)), err
}
func readLock(b []byte) (lockRecord, error) {
	var r lockRecord
	err := json.Unmarshal(b, &r)
	return r, err
}
func writeLockRecord(f *os.File, r lockRecord) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if _, err = f.WriteAt(append(b, '\n'), 0); err != nil {
		return err
	}
	return f.Truncate(int64(len(b) + 1))
}
func AdoptLock(f *os.File) (*Lease, error) {
	l := &Lease{File: f}
	start, err := runner.OwnerStart()
	if err != nil {
		return nil, err
	}
	command, err := processCommand(os.Getpid())
	if err != nil {
		return nil, err
	}
	if err = writeLockRecord(f, lockRecord{os.Getpid(), start, command}); err != nil {
		return nil, err
	}
	return l, nil
}
func Lock(data string) (*Lease, error) {
	f, err := os.OpenFile(filepath.Join(data, "daemon.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		b, _ := os.ReadFile(f.Name())
		f.Close()
		if err == syscall.EWOULDBLOCK {
			r, _ := readLock(b)
			return nil, &LockedError{strconv.Itoa(r.PID)}
		}
		return nil, err
	}
	l, err := AdoptLock(f)
	if err != nil {
		f.Close()
	}
	return l, err
}

// Holder checks the recorded owner identity without ever taking the lock.
func Holder(data string) int {
	b, err := os.ReadFile(filepath.Join(data, "daemon.lock"))
	if err != nil {
		return 0
	}
	r, err := readLock(b)
	if err != nil || r.PID <= 0 {
		return 0
	}
	start, err := runner.ProcessStart(r.PID)
	if err != nil || !start.Equal(r.Start) {
		return 0
	}
	command, err := processCommand(r.PID)
	if err != nil || command != r.Command {
		return 0
	}
	return r.PID
}
