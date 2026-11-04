package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestReleaseDocumentation(t *testing.T) {
	raw, err := os.ReadFile("../../CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, term := range []string{"M7:", "Skeptic round 1"} {
		if !strings.Contains(string(raw), term) {
			t.Errorf("missing %s", term)
		}
	}
	for n := 17; n <= 28; n++ {
		if !strings.Contains(string(raw), fmt.Sprintf("#%d", n)) {
			t.Errorf("missing #%d", n)
		}
	}
	plan, err := os.ReadFile("../../docs/PLAN.md")
	if err != nil {
		t.Fatal(err)
	}
	// Pin the fact that PLAN and AGENTS agree, not the status wording.
	agents, err := os.ReadFile("../../AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	_, after, ok := strings.Cut(string(agents), "## Status\n")
	status, _, _ := strings.Cut(after, "\n")
	if !ok || status == "" || !strings.Contains(string(plan), "Status: "+status) {
		t.Errorf("PLAN status does not match AGENTS.md status %q", status)
	}
}
