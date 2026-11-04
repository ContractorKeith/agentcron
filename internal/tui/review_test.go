package tui

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/ContractorKeith/agentcron/internal/app"
)

func TestSuccessfulRefreshClearsTransientError(t *testing.T) {
	m, f := fixture()
	f.err = errors.New("ledger unavailable")
	m = deliver(m, m.load())
	if !strings.Contains(m.render(), "ledger unavailable") {
		t.Fatal("missing refresh error")
	}
	f.err = nil
	m = deliver(m, m.load())
	if strings.Contains(m.render(), "ledger unavailable") {
		t.Fatal("successful refresh left stale error in footer")
	}
}

func TestApproveSelectedUnapprovedJobRefreshesStatus(t *testing.T) {
	m, f := fixture()
	f.jobs = append(f.jobs, app.JobView{Name: "proposed", Status: "unapproved"})
	m = deliver(m, m.load())
	m.api = approving{f}
	m, cmd := press(m, "j")
	m = deliver(m, cmd)
	m, cmd = press(m, "a")
	next, refresh := m.Update(cmd())
	m = next.(model)
	if f.calls[len(f.calls)-1] != "approve proposed" {
		t.Fatalf("wrong approval: %v", f.calls)
	}
	f.jobs = append([]app.JobView(nil), f.jobs...)
	f.jobs[1].Status = "active"
	if m.visibleJobs()[m.jobIndex].Status != "unapproved" {
		t.Fatal("fake response aliased current model")
	}
	m = deliver(m, refresh)
	if m.visibleJobs()[m.jobIndex].Status != "active" {
		t.Fatal("approval status did not refresh")
	}
}

func TestCancelPromptIgnoresUnrelatedKeys(t *testing.T) {
	for _, key := range []string{"q", "j", "?", "r", "tab", "ctrl+c"} {
		t.Run(key, func(t *testing.T) {
			m, f := fixture()
			m, _ = press(m, "tab")
			m, _ = press(m, "c")
			count := len(f.calls)
			m, cmd := press(m, key)
			if cmd != nil || m.confirm != "r_one" || len(f.calls) != count {
				t.Fatal("unrelated key escaped cancellation prompt")
			}
		})
	}
	for _, key := range []string{"n", "esc"} {
		t.Run(key, func(t *testing.T) {
			m, _ := fixture()
			m, _ = press(m, "tab")
			m, _ = press(m, "c")
			m, cmd := press(m, key)
			if m.confirm != "" || cmd != nil {
				t.Fatal("declining cancellation did not close prompt")
			}
		})
	}
}

func TestCarriageReturnsSeparateLogLines(t *testing.T) {
	m, _ := fixture()
	m.logID = "r_one"
	m.output = "first\rsecond\rthird"
	if !strings.Contains(m.render(), "first\nsecond\nthird") {
		t.Fatal("carriage returns merged output lines")
	}
}

func TestInitialFrameChecksDaemonBeforeReportingAbsent(t *testing.T) {
	ready, f := fixture()
	m := New(ready.ctx, f, ready.clock).(model)
	first := m.View().Content
	if !strings.Contains(first, "checking daemon…") || strings.Contains(first, "daemon not running") {
		t.Fatal("initial frame asserts daemon absence before checking")
	}
	m = deliver(m, m.load())
	if !strings.Contains(m.render(), "daemon not running") || strings.Contains(m.render(), "checking daemon…") {
		t.Fatal("checked daemon status not displayed")
	}
}

func TestRunIDTruncationPreservesRunes(t *testing.T) {
	if got := runID("r_ééé", 4); got != "…ééé" {
		t.Fatalf("truncation split a rune: %q", got)
	}
	if got := runID("r_ééé", 3); got != "…éé" {
		t.Fatalf("invalid rune truncation: %q", got)
	}
	if got := runID("éé", 2); got != "éé" {
		t.Fatalf("short rune ID changed: %q", got)
	}
}

func TestReadmeDescribesPreRelease(t *testing.T) {
	text, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "**Status:** pre-release; see [the plan]") || strings.Contains(string(text), "planned, no code yet") {
		t.Fatal("README still describes an unimplemented project")
	}
}

func TestCancelPromptShowsOnlyAvailableKeys(t *testing.T) {
	m, _ := fixture()
	m, _ = press(m, "tab")
	m, _ = press(m, "c")
	view := m.render()
	if strings.Contains(view, "q quit") || !strings.Contains(view, "n/esc back") {
		t.Fatal("confirmation footer advertises unavailable keys")
	}
}
