package main

import (
	"os"
	"strings"
	"testing"
)

func TestContainmentDocumentation(t *testing.T) {
	for _, path := range []string{"README.md", "docs/PLAN.md", "docs/adr/0003-unattended-safety-defaults.md"} {
		raw, err := os.ReadFile("../../" + path)
		if err != nil {
			t.Fatal(err)
		}
		for _, term := range []string{"setsid", "setpgid", "job control", "cgroup"} {
			if !strings.Contains(string(raw), term) {
				t.Errorf("%s missing %s", path, term)
			}
		}
	}
}
