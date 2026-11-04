// Package tui implements the interactive jobs, runs and output views.
package tui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/ContractorKeith/agentcron/internal/clock"
)

// Backend is the small application surface used by the model.
type Backend interface {
	Jobs(context.Context) ([]app.JobView, error)
	Runs(context.Context, string) ([]app.Run, error)
	Tail(context.Context, string, bool) (string, error)
	DaemonRunning(context.Context) (bool, error)
	Enqueue(context.Context, string) error
	Pause(context.Context, string, bool) error
	Cancel(context.Context, string) error
}

// Approver enables human approval of agent-created jobs.
type Approver interface {
	Approve(context.Context, string) error
}

type model struct {
	ctx                                    context.Context
	api                                    Backend
	clock                                  clock.Clock
	jobs                                   []app.JobView
	runs                                   []app.Run
	jobIndex, runIndex, pane               int
	width, height                          int
	filter, output, logID, confirm, footer string
	filtering, help, stderr, daemon, busy  bool
	loading, refreshError, daemonChecked   bool
	generation                             int
}
type tickMsg struct{}
type actionMsg struct{ err error }
type dataMsg struct {
	generation         int
	jobs               []app.JobView
	runs               []app.Run
	job, logID, output string
	stderr, daemon     bool
	err                error
}

func New(ctx context.Context, api Backend, c clock.Clock) tea.Model {
	return model{ctx: ctx, api: api, clock: c, width: 80, height: 24, loading: true}
}
func (m model) Init() tea.Cmd { return tea.Batch(m.load(), m.tick()) }
func (m model) tick() tea.Cmd {
	return func() tea.Msg {
		timer := m.clock.After(time.Second)
		select {
		case <-m.ctx.Done():
			return nil
		case <-timer:
			return tickMsg{}
		}
	}
}
func (m model) load() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
		defer cancel()
		d := dataMsg{generation: m.generation, job: m.jobName(), logID: m.logID, stderr: m.stderr}
		d.jobs, d.err = m.api.Jobs(ctx)
		if d.err == nil {
			d.daemon, d.err = m.api.DaemonRunning(ctx)
		}
		if d.err == nil && d.job != "" {
			d.runs, d.err = m.api.Runs(ctx, d.job)
		}
		if d.err == nil && d.logID != "" {
			d.output, d.err = m.api.Tail(ctx, d.logID, d.stderr)
		}
		return d
	}
}
func (m model) visibleJobs() []app.JobView {
	var out []app.JobView
	for _, j := range m.jobs {
		if strings.Contains(strings.ToLower(j.Name), strings.ToLower(m.filter)) {
			out = append(out, j)
		}
	}
	return out
}
func (m model) jobName() string {
	js := m.visibleJobs()
	if m.jobIndex >= 0 && m.jobIndex < len(js) {
		return js[m.jobIndex].Name
	}
	return ""
}
func (m *model) reload() tea.Cmd { m.generation++; m.loading = true; return m.load() }
func (m *model) selectionChanged() tea.Cmd {
	m.runs = nil
	m.runIndex = 0
	return m.reload()
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, v.Width), max(1, v.Height)
	case tickMsg:
		if m.loading {
			return m, m.tick()
		}
		cmd := m.reload()
		return m, tea.Batch(cmd, m.tick())
	case dataMsg:
		if v.generation != m.generation {
			return m, nil
		}
		m.loading = false
		if v.err != nil {
			m.footer = v.err.Error()
			m.refreshError = true
			return m, nil
		}
		if m.refreshError {
			m.footer = ""
			m.refreshError = false
		}
		name := m.jobName()
		m.jobs, m.daemon, m.daemonChecked = v.jobs, v.daemon, true
		js := m.visibleJobs()
		m.jobIndex = min(m.jobIndex, max(0, len(js)-1))
		for i, j := range js {
			if j.Name == name {
				m.jobIndex = i
				break
			}
		}
		if v.job != m.jobName() {
			cmd := m.selectionChanged()
			return m, cmd
		}
		id := ""
		if m.runIndex < len(m.runs) {
			id = m.runs[m.runIndex].ID
		}
		m.runs = v.runs
		m.runIndex = min(m.runIndex, max(0, len(m.runs)-1))
		for i, r := range m.runs {
			if r.ID == id {
				m.runIndex = i
				break
			}
		}
		if v.logID == m.logID && v.stderr == m.stderr {
			m.output = v.output
		}
	case actionMsg:
		m.refreshError = false
		m.busy = false
		if v.err != nil {
			m.footer = v.err.Error()
		} else {
			m.footer = "done"
		}
		cmd := m.reload()
		return m, cmd
	case tea.KeyPressMsg:
		return m.key(v)
	}
	return m, nil
}
