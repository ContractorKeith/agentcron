package tui

import (
	"strings"
	"testing"
)

func TestApprovalConfirmationDetails(t *testing.T) {
	m, f := fixture()
	m.api = approving{f}
	m.width = 240
	j := m.jobs[0].Job
	j.Runner = "command"
	j.Command = []string{"sh", "-c", "echo ok"}
	j.Args = []string{"--arg"}
	j.Repo = "/repo"
	j.Prompt = "First line\nsecond line"
	before := len(f.calls)
	m, cmd := press(m, "a")
	if cmd != nil || m.busy {
		t.Fatal("approved without confirmation")
	}
	view := m.render()
	for _, term := range []string{"command", "sh", "echo ok", "--arg", "/repo", "First line", "y/n"} {
		if !strings.Contains(view, term) {
			t.Errorf("missing %s: %s", term, view)
		}
	}
	m, cmd = press(m, "n")
	m = deliver(m, cmd)
	if len(f.calls) != before {
		t.Fatal(f.calls)
	}
	m, _ = press(m, "a")
	m, cmd = press(m, "y")
	m = deliver(m, cmd)
	if f.calls[len(f.calls)-1] != "approve daily" {
		t.Fatal(f.calls)
	}
}
