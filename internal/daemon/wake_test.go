package daemon

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
)

func TestWakeRejectsUnlockedAndStaleIdentity(t *testing.T) {
	dir := t.TempDir()
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)
	lease, err := Lock(dir)
	check(t, err)
	raw, err := os.ReadFile(lease.Name())
	check(t, err)
	r, err := readLock(raw)
	check(t, err)
	r.Command = "wrong"
	check(t, writeLockRecord(lease.File, r))
	if Wake(dir) {
		t.Fatal("signalled changed identity")
	}
	r.Command, _ = processCommand(os.Getpid())
	check(t, writeLockRecord(lease.File, r))
	// Keep a valid identity in an unlocked file: the record alone is insufficient.
	lease.Transfer()
	check(t, lease.Close())
	if Wake(dir) {
		t.Fatal("signalled without lock")
	}
}
