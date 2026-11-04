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
	if !strings.Contains(string(plan), "Status: v0.1.0 release candidate; skeptic-review pending") {
		t.Error("stale PLAN status")
	}
}
