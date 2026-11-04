package tui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m model) key(v tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := v.String()
	if m.confirm != "" {
		switch key {
		case "y":
			id := m.confirm
			m.confirm = ""
			return m.action(func(ctx context.Context) error { return m.api.Cancel(ctx, id) })
		case "n", "esc":
			m.confirm = ""
		}
		return m, nil
	}
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.filtering {
		switch key {
		case "enter":
			m.filtering = false
		case "esc":
			m.filtering = false
			m.filter = ""
		case "backspace":
			rs := []rune(m.filter)
			if len(rs) > 0 {
				m.filter = string(rs[:len(rs)-1])
			}
		default:
			m.filter += v.Text
		}
		m.jobIndex = 0
		cmd := m.selectionChanged()
		return m, cmd
	}
	if key == "q" {
		return m, tea.Quit
	}
	if key == "?" {
		m.help = !m.help
		return m, nil
	}
	if m.help {
		if key == "esc" {
			m.help = false
		}
		return m, nil
	}
	if m.logID != "" {
		switch key {
		case "esc":
			m.logID = ""
			m.output = ""
			cmd := m.reload()
			return m, cmd
		case "tab", "s":
			m.stderr = !m.stderr
			m.output = ""
			cmd := m.reload()
			return m, cmd
		}
		return m, nil
	}
	switch key {
	case "/":
		m.filtering = true
		m.pane = 0
	case "esc":
		m.filter = ""
		m.jobIndex = 0
		cmd := m.selectionChanged()
		return m, cmd
	case "tab":
		m.pane = 1 - m.pane
	case "j", "down", "k", "up":
		delta := 1
		if key == "k" || key == "up" {
			delta = -1
		}
		if m.pane == 0 {
			m.jobIndex = max(0, min(m.jobIndex+delta, len(m.visibleJobs())-1))
			cmd := m.selectionChanged()
			return m, cmd
		}
		m.runIndex = max(0, min(m.runIndex+delta, len(m.runs)-1))
	case "enter":
		if len(m.runs) > 0 {
			m.logID = m.runs[m.runIndex].ID
			m.stderr = false
			cmd := m.reload()
			return m, cmd
		}
	case "c":
		if m.pane == 1 && len(m.runs) > 0 && m.runs[m.runIndex].State == "running" {
			m.confirm = m.runs[m.runIndex].ID
		}
	case "r", "p", "a":
		name := m.jobName()
		if name == "" || m.busy {
			return m, nil
		}
		switch key {
		case "r":
			return m.action(func(ctx context.Context) error { return m.api.Enqueue(ctx, name) })
		case "p":
			j := m.visibleJobs()[m.jobIndex]
			paused := j.State != nil && j.State.Paused
			return m.action(func(ctx context.Context) error { return m.api.Pause(ctx, name, !paused) })
		case "a":
			if api, ok := m.api.(Approver); ok {
				return m.action(func(ctx context.Context) error { return api.Approve(ctx, name) })
			}
			m.footer = "approve not available"
		}
	}
	return m, nil
}

func (m model) action(fn func(context.Context) error) (tea.Model, tea.Cmd) {
	if m.busy {
		return m, nil
	}
	m.busy = true
	m.footer = "working…"
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 15*time.Second)
		defer cancel()
		return actionMsg{err: fn(ctx)}
	}
}

// plain prevents log output and user-controlled fields from injecting terminal controls.
func plain(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Map(func(r rune) rune {
		if r == '\r' {
			return '\n'
		}
		if r == '\n' || r >= 32 && r != 127 && !(r >= 0x80 && r < 0xa0) {
			return r
		}
		return -1
	}, s)
}
