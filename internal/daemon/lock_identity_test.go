package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHolderCanReadWithoutWriteAccess(t *testing.T) {
	data := t.TempDir()
	lease, err := Lock(data)
	check(t, err)
	defer lease.Close()
	path := filepath.Join(data, "daemon.lock")
	check(t, os.Chmod(path, 0400))
	defer os.Chmod(path, 0600)
	if got := Holder(data); got != os.Getpid() {
		t.Fatalf("read-only probe returned %d", got)
	}
}
func TestHolderRejectsChangedProcessIdentity(t *testing.T) {
	data := t.TempDir()
	lease, err := Lock(data)
	check(t, err)
	defer lease.Close()
	b, err := os.ReadFile(filepath.Join(data, "daemon.lock"))
	check(t, err)
	// A stale lock record with a live PID must not identify its replacement as the owner.
	record, err := readLock(b)
	check(t, err)
	record.Start = record.Start.Add(-time.Hour)
	check(t, writeLockRecord(lease.File, record))
	if got := Holder(data); got != 0 {
		t.Fatalf("recycled holder reported as %d", got)
	}
}
