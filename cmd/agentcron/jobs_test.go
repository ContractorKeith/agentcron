package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ContractorKeith/agentcron/internal/app"
)

func cli(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}
func TestJobCommands(t *testing.T) {
	isolate(t)
	now := []string{"--now", "2026-11-04T12:00:00Z"}
	if _, err := cli(t, "ls", "--json"); app.Code(err) != app.NotInitialized {
		t.Fatal(err)
	}
	if _, err := cli(t, "init"); err != nil {
		t.Fatal(err)
	}
	args := []string{"add", "echoer", "--schedule", "@daily", "--runner", "command", "--command", "cat", "--command", "-n", "--arg", "--literal", "--prompt", "hi", "--no-memory"}
	if out, err := cli(t, append(append([]string{}, args...), "--dry-run")...); err != nil || !strings.Contains(out, "command = [\"cat\", \"-n\"]") {
		t.Fatal(out, err)
	}
	if _, err := cli(t, append(args, now...)...); err != nil {
		t.Fatal(err)
	}
	out, err := cli(t, append([]string{"show", "echoer", "--json"}, now...)...)
	if err != nil {
		t.Fatal(err)
	}
	var detail app.JobDetail
	if err := json.Unmarshal([]byte(out), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Job.Memory || len(detail.Job.Command) != 2 || detail.Job.Command[1] != "-n" || len(detail.Job.Args) != 1 {
		t.Fatal(detail)
	}
	for _, args := range [][]string{
		{"pause", "unknown"}, {"resume", "unknown"}, {"rm", "unknown"}, {"show", "unknown"},
		{"ls", "extra"}, {"add"}, {"show"}, {"next", "--count", "0"}, {"ls", "--now", "bad"},
		{"add", "bad", "--schedule", "@daily", "--runner", "command", "--prompt", "hi"},
	} {
		if _, err := cli(t, args...); app.Code(err) != app.Usage {
			t.Fatalf("%v: %v", args, err)
		}
	}
	out, err = cli(t, "--help")
	if err != nil || strings.Contains(out, "--now") {
		t.Fatal(out, err)
	}
}
