package app

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
)

func queuedLogFixture(t *testing.T) (Options, Run) {
	t.Helper()
	opts := isolated(t)
	if err := Init(t.Context(), opts); err != nil {
		t.Fatal(err)
	}
	j := NewJob("logs")
	j.Schedule = "@daily"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hi"
	if _, err := AddJob(t.Context(), opts, clock.Real{}, j, false); err != nil {
		t.Fatal(err)
	}
	r, err := RunJob(t.Context(), opts, clock.Real{}, j.Name, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(opts.DataDir, "runs", r.ID), 0700); err != nil {
		t.Fatal(err)
	}
	return opts, r
}

func TestTailRejectsFIFOWithoutBlocking(t *testing.T) {
	opts, r := queuedLogFixture(t)
	path := filepath.Join(opts.DataDir, "runs", r.ID, "stdout.log")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := TailOutput(t.Context(), opts, clock.Real{}, r.ID, "stdout", 16); done <- err }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "log is not a regular file") {
			t.Fatal(err)
		}
	case <-time.After(500 * time.Millisecond):
		// Release the old blocking open so a failing test still reaps its reader.
		fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_NONBLOCK, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer syscall.Close(fd)
		<-done
		t.Fatal("FIFO blocked the output reader")
	}
}

func TestTailRejectsSymlinkAndDirectory(t *testing.T) {
	for _, kind := range []string{"symlink", "directory", "escaped-run"} {
		t.Run(kind, func(t *testing.T) {
			opts, r := queuedLogFixture(t)
			dir := filepath.Join(opts.DataDir, "runs", r.ID)
			path := filepath.Join(dir, "stdout.log")
			outside := t.TempDir()
			if err := os.WriteFile(filepath.Join(outside, "stdout.log"), []byte("private"), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink(filepath.Join(outside, "stdout.log"), path)
			case "directory":
				err = os.Mkdir(path, 0700)
			case "escaped-run":
				if err = os.Remove(dir); err == nil {
					err = os.Symlink(outside, dir)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			out, err := TailOutput(t.Context(), opts, clock.Real{}, r.ID, "stdout", 16)
			if err == nil || out.Output != "" {
				t.Fatalf("unsafe log read: %+v %v", out, err)
			}
			if kind != "escaped-run" && !strings.Contains(err.Error(), "log is not a regular file") {
				t.Fatal(err)
			}
		})
	}
}

func TestDoctorMissingJobsDirectory(t *testing.T) {
	opts := isolated(t)
	if err := Init(t.Context(), opts); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(opts.ConfigDir, "jobs")); err != nil {
		t.Fatal(err)
	}
	health, err := Doctor(t.Context(), opts)
	if err != nil || !health.ConfigDir || !health.DataDir {
		t.Fatal(health, err)
	}
}

func TestUnapprovedManualRunRefusedWithoutLedgerRow(t *testing.T) {
	opts := isolated(t)
	c := clock.Real{}
	if err := Init(t.Context(), opts); err != nil {
		t.Fatal(err)
	}
	j := NewJob("unapproved")
	j.Schedule = "@daily"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hi"
	if _, err := CreateMCPJob(t.Context(), opts, c, j); err != nil {
		t.Fatal(err)
	}
	for _, now := range []bool{false, true} {
		_, err := RunJob(t.Context(), opts, c, j.Name, now, false)
		if Code(err) != Refused {
			t.Errorf("now=%t: got %v, code %d; want refused", now, err, Code(err))
		}
	}
	runs, err := Runs(t.Context(), opts, c, j.Name, "", 50)
	if err != nil || len(runs) != 0 {
		t.Fatal("refused runs wrote ledger rows", runs, err)
	}
}
