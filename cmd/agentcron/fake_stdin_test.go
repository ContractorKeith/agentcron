package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestSleepingFakeWithOpenStdin(t *testing.T) {
	scratch := isolate(t)
	binDir := filepath.Join(scratch, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	build(t, filepath.Join(binDir, "claude"), "../../testdata/fakes/claude")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_STDIN", "")
	t.Setenv("FAKE_MODE", "sleep")
	t.Setenv("FAKE_SLEEP", "1m")
	record := filepath.Join(scratch, "record.jsonl")
	t.Setenv("FAKE_RECORD", record)
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer write.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if found, err := exec.LookPath("claude"); err != nil || found != filepath.Join(binDir, "claude") {
		t.Fatalf("fake not first: %q %v", found, err)
	}
	cmd := testCommand(ctx, "claude")
	cmd.Stdin = read // Keep the writer open until after the process is reaped.
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	contents, recordErr := waitForFakeRecord(record)
	signalErr := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	waitErr := cmd.Wait()
	if signalErr != nil {
		t.Fatal(signalErr)
	}
	var exit *exec.ExitError
	if !errors.As(waitErr, &exit) || exit.ProcessState.Sys().(syscall.WaitStatus).Signal() != syscall.SIGTERM {
		t.Fatalf("expected SIGTERM exit, got %v", waitErr)
	}
	if recordErr != nil {
		t.Fatalf("fake blocked before entering sleep with stdin open: %v", recordErr)
	}
	var got struct{ Stdin string }
	if err := json.Unmarshal(contents, &got); err != nil {
		t.Fatal(err)
	}
	if got.Stdin != "" {
		t.Fatalf("stdin = %q", got.Stdin)
	}
}

func waitForFakeRecord(path string) ([]byte, error) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for {
		if contents, err := os.ReadFile(path); err == nil && len(contents) > 0 {
			return contents, nil
		} else if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			return nil, os.ErrDeadlineExceeded
		}
	}
}
