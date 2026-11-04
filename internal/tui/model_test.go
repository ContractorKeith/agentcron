package tui

import (
	"context"
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/ContractorKeith/agentcron/internal/clock"
)

type fake struct {
	jobs   []app.JobView
	runs   []app.Run
	calls  []string
	err    error
	output string
}

func (f *fake) Jobs(context.Context) ([]app.JobView, error) { return f.jobs, f.err }
func (f *fake) Runs(_ context.Context, n string) ([]app.Run, error) {
	f.calls = append(f.calls, "runs "+n)
	return f.runs, f.err
}
func (f *fake) Tail(_ context.Context, id string, stderr bool) (string, error) {
	f.calls = append(f.calls, fmt.Sprintf("tail %s %t", id, stderr))
	return f.output, f.err
}
func (f *fake) DaemonRunning(context.Context) (bool, error) { return false, f.err }
func (f *fake) Enqueue(_ context.Context, n string) error {
	f.calls = append(f.calls, "enqueue "+n)
	return f.err
}
func (f *fake) Pause(_ context.Context, n string, p bool) error {
	f.calls = append(f.calls, fmt.Sprintf("pause %s %t", n, p))
	return f.err
}
func (f *fake) Cancel(_ context.Context, id string) error {
	f.calls = append(f.calls, "cancel "+id)
	return f.err
}

type approving struct{ *fake }

func (f approving) Approve(_ context.Context, n string) error {
	f.calls = append(f.calls, "approve "+n)
	return f.err
}

func fixture() (model, *fake) {
	c := clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
	j := app.NewJob("daily")
	j.Schedule = "@daily"
	f := &fake{jobs: []app.JobView{{Name: "daily", Job: &j, Status: "active"}}, runs: []app.Run{{ID: "r_one", Job: "daily", State: "running", Trigger: "manual"}}, output: "first\nsecond\n"}
	m := New(context.Background(), f, c).(model)
	m = deliver(m, m.load())
	m = deliver(m, m.load())
	return m, f
}
func deliver(m model, cmd tea.Cmd) model {
	if cmd == nil {
		return m
	}
	next, _ := m.Update(cmd())
	return next.(model)
}
func press(m model, key string) (model, tea.Cmd) {
	codes := map[string]rune{"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab, "down": tea.KeyDown, "up": tea.KeyUp, "backspace": tea.KeyBackspace}
	v := tea.KeyPressMsg{Code: codes[key]}
	if key == "ctrl+c" {
		v.Code = 'c'
		v.Mod = tea.ModCtrl
	} else if v.Code == 0 {
		v.Code = []rune(key)[0]
		v.Text = key
	}
	next, cmd := m.Update(v)
	return next.(model), cmd
}
func TestActions(t *testing.T) {
	for _, tc := range []struct{ key, want string }{{"r", "enqueue daily"}, {"p", "pause daily true"}, {"a", "approve daily"}} {
		t.Run(tc.key, func(t *testing.T) {
			m, f := fixture()
			m.api = approving{f}
			m, cmd := press(m, tc.key)
			if tc.key == "a" {
				m, cmd = press(m, "y")
			}
			if !m.busy {
				t.Fatal("action did not become busy")
			}
			_, duplicate := press(m, tc.key)
			if duplicate != nil {
				t.Fatal("duplicate action while busy")
			}
			m = deliver(m, cmd)
			if m.busy || f.calls[len(f.calls)-1] != tc.want {
				t.Fatalf("calls %v busy %v", f.calls, m.busy)
			}
		})
	}
	m, _ := fixture()
	m, _ = press(m, "a")
	if m.footer != "approve not available" {
		t.Fatal(m.footer)
	}
}
func TestCancelConfirmation(t *testing.T) {
	m, f := fixture()
	m, _ = press(m, "tab")
	m, _ = press(m, "c")
	if m.confirm != "r_one" {
		t.Fatal("no confirmation")
	}
	before := len(f.calls)
	m, cmd := press(m, "n")
	m = deliver(m, cmd)
	if len(f.calls) != before {
		t.Fatal("cancelled after n")
	}
	m, _ = press(m, "c")
	m, cmd = press(m, "y")
	m = deliver(m, cmd)
	if f.calls[len(f.calls)-1] != "cancel r_one" {
		t.Fatal(f.calls)
	}
	m.runs[0].State = "succeeded"
	m, _ = press(m, "c")
	if m.confirm != "" {
		t.Fatal("terminal run cancellation")
	}
}
func TestNavigationLogsAndFiltering(t *testing.T) {
	m, f := fixture()
	f.jobs = append(f.jobs, app.JobView{Name: "weekly", Status: "paused"})
	m = deliver(m, m.load())
	m, cmd := press(m, "down")
	m = deliver(m, cmd)
	if m.jobName() != "weekly" || f.calls[len(f.calls)-1] != "runs weekly" {
		t.Fatal(f.calls, m.jobName())
	}
	m, cmd = press(m, "up")
	m = deliver(m, cmd)
	m, _ = press(m, "tab")
	m, cmd = press(m, "enter")
	m = deliver(m, cmd)
	if m.logID != "r_one" || m.output != f.output {
		t.Fatal("logs not opened")
	}
	m, cmd = press(m, "tab")
	m = deliver(m, cmd)
	if !m.stderr || f.calls[len(f.calls)-1] != "tail r_one true" {
		t.Fatal(f.calls)
	}
	f.output = "live\n"
	m = deliver(m, m.load())
	if m.output != "live\n" {
		t.Fatal("tail did not follow")
	}
	m, cmd = press(m, "esc")
	m = deliver(m, cmd)
	m, _ = press(m, "/")
	m, cmd = press(m, "w")
	m = deliver(m, cmd)
	if m.jobName() != "weekly" || len(m.visibleJobs()) != 1 {
		t.Fatal("filter failed")
	}
	m, _ = press(m, "q")
	if !m.filtering || m.filter != "wq" {
		t.Fatal("filter consumed q as quit")
	}
	_, cmd = press(m, "ctrl+c")
	if cmd == nil || cmd() != tea.Quit() {
		t.Fatal("ctrl-c not quit")
	}
	m, _ = press(m, "esc")
	m, _ = press(m, "?")
	m, cmd = press(m, "r")
	if cmd != nil || !m.help {
		t.Fatal("help leaked action")
	}
}

func TestEnterOpensLogsFromJobsPane(t *testing.T) {
	m, _ := fixture()
	m, cmd := press(m, "enter")
	m = deliver(m, cmd)
	if m.logID != "r_one" {
		t.Fatal("enter did not open selected run logs")
	}
}
