package main

import (
	"os"
	"regexp"
	"testing"
)

func TestCIUsesReleasedStaticcheck(t *testing.T) {
	isolate(t)
	workflow, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	// Accept release updates, while refusing latest, branches and pseudo-versions.
	pin := regexp.MustCompile(`(?m)^\s*- run: go install honnef\.co/go/tools/cmd/staticcheck@v[0-9]+\.[0-9]+\.[0-9]+\s*$`)
	if !pin.Match(workflow) {
		t.Fatal("CI must install a pinned staticcheck release")
	}
}
