package daemon

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestWakeUsesIdentityWithoutAcquiringLease(t *testing.T) {
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
	// An identity-valid record is sufficient; waking must not acquire the lease.
	lease.Transfer()
	check(t, lease.Close())
	if !Wake(dir) {
		t.Fatal("identity-valid wake failed")
	}
	select {
	case <-hup:
	case <-time.After(time.Second):
		t.Fatal("no HUP")
	}
	next, err := Lock(dir)
	check(t, err)
	check(t, next.Close())
}
