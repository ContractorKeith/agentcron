package main

import (
	"strings"
	"testing"
)

func TestQueueWithoutDaemonMessage(t *testing.T) {
	isolate(t)
	if _, err := cli(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := cli(t, "add", "job", "--schedule", "@daily", "--runner", "command", "--command", "cat", "--prompt", "hi", "--no-memory"); err != nil {
		t.Fatal(err)
	}
	out, err := cli(t, "run", "job")
	if err != nil || !strings.Contains(out, "queued; no daemon running") {
		t.Fatal(out, err)
	}
}
