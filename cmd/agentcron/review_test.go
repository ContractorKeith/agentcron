package main

import (
	"strings"
	"testing"

	"github.com/ContractorKeith/agentcron/internal/app"
)

func TestPinnedClockRefusesWrites(t *testing.T) {
	isolate(t)
	if _, err := cli(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := cli(t, "add", "example", "--schedule", "@daily", "--runner", "claude", "--prompt", "hi"); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "new", "--schedule", "@daily", "--runner", "claude", "--prompt", "hi"},
		{"add", "dry", "--schedule", "@daily", "--runner", "claude", "--prompt", "hi", "--dry-run"},
		{"pause", "example"}, {"resume", "example"}, {"rm", "example"}, {"rm", "example", "--dry-run"},
	} {
		out, err := cli(t, append(args, "--now", "2020-01-01T00:00:00Z")...)
		if app.Code(err) != app.Usage || !strings.Contains(err.Error(), "now") {
			t.Errorf("%v: %s %v", args, out, err)
		}
	}
}
func TestHumanNextAndRemove(t *testing.T) {
	isolate(t)
	if _, err := cli(t, "init"); err != nil {
		t.Fatal(err)
	}
	args := []string{"next", "--schedule", "@daily", "--timezone", "UTC", "--count", "2", "--now", "2026-11-04T12:00:00Z"}
	if out, err := cli(t, args...); err != nil || out != "2026-11-05T00:00:00Z\n2026-11-06T00:00:00Z\n" {
		t.Errorf("%q %v", out, err)
	}
	if _, err := cli(t, "add", "example", "--schedule", "@daily", "--timezone", "UTC", "--runner", "claude", "--prompt", "hi"); err != nil {
		t.Fatal(err)
	}
	if out, err := cli(t, "next", "--count", "1", "--now", "2026-11-04T12:00:00Z"); err != nil || out != "2026-11-05T00:00:00Z  example\n" {
		t.Errorf("%q %v", out, err)
	}
	if out, err := cli(t, "rm", "example", "--dry-run"); err != nil || out != "Would remove example\n" {
		t.Errorf("%q %v", out, err)
	}
	if out, err := cli(t, "rm", "example"); err != nil || out != "Removed example\n" {
		t.Errorf("%q %v", out, err)
	}
}
func TestPinnedOutputUTC(t *testing.T) {
	isolate(t)
	if _, err := cli(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := cli(t, "add", "example", "--schedule", "@daily", "--runner", "claude", "--prompt", "hi"); err != nil {
		t.Fatal(err)
	}
	for _, command := range [][]string{{"show", "example"}, {"ls"}} {
		out, err := cli(t, append(command, "--json", "--now", "2026-11-04T07:00:00-05:00")...)
		if err != nil || !strings.Contains(out, `"updated_at":"2026-11-04T12:00:00Z"`) {
			t.Errorf("%s %v", out, err)
		}
	}
}
