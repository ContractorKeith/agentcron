package main

import (
	"strings"
	"testing"
)

func TestAddCatchupFlag(t *testing.T) {
	isolate(t)
	if _, err := cli(t, "init"); err != nil {
		t.Fatal(err)
	}
	for _, policy := range []string{"one", "none"} {
		out, err := cli(t, "add", "job", "--schedule", "@daily", "--runner", "command", "--command", "cat", "--prompt", "hi", "--no-memory", "--catchup", policy, "--dry-run")
		if err != nil || !strings.Contains(out, `catchup = "`+policy+`"`) {
			t.Fatal(out, err)
		}
	}
}
