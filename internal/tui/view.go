package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ContractorKeith/agentcron/internal/app"
)

func (m model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}
func (m model) render() string {
	lines := []string{"agentcron"}
	if !m.daemon {
		lines = append(lines, "daemon not running")
	}
	switch {
	case m.help:
		lines = append(lines, "j/k ↑/↓ move; tab switch jobs/runs", "enter runs/logs; esc back", "r enqueue; p pause/resume; a approve", "c cancel running run (y/n)", "/ filter jobs; ? help; q/ctrl-c quit", "Logs: tab/s toggle stdout/stderr; live tail (64 KiB)")
	case m.logID != "":
		stream := "stdout"
		if m.stderr {
			stream = "stderr"
		}
		lines = append(lines, "Logs "+m.logID+" — "+stream+" (live tail)")
		out := strings.Split(strings.TrimSuffix(plain(m.output), "\n"), "\n")
		room := max(0, m.height-len(lines)-2)
		if len(out) > room {
			out = out[len(out)-room:]
		}
		lines = append(lines, out...)
	default:
		room := max(0, m.height-len(lines)-4)
		jobRoom := room / 2
		runRoom := room - jobRoom
		jobTitle, runTitle := "Jobs", "Runs — "+m.jobName()
		if m.pane == 0 {
			jobTitle = "> " + jobTitle
		} else {
			runTitle = "> " + runTitle
		}
		lines = append(lines, jobTitle+"  name | schedule | next | last | status")
		js := m.visibleJobs()
		if len(js) == 0 {
			lines = append(lines, "  No jobs")
		} else {
			start, end := window(len(js), m.jobIndex, jobRoom)
			for i := start; i < end; i++ {
				lines = append(lines, marker(i == m.jobIndex)+jobRow(js[i], m.width))
			}
		}
		lines = append(lines, runTitle+"  id | trigger | state | started | duration")
		if len(m.runs) == 0 {
			lines = append(lines, "  No runs")
		} else {
			start, end := window(len(m.runs), m.runIndex, runRoom)
			for i := start; i < end; i++ {
				lines = append(lines, marker(i == m.runIndex)+m.runRow(m.runs[i]))
			}
		}
	}
	footer := m.footer
	if m.filtering || m.filter != "" {
		footer = "Filter: " + m.filter + "  " + footer
	}
	if m.confirm != "" {
		footer = "Cancel " + m.confirm + "? y/n"
	}
	// Keep the footer fixed and every frame inside the terminal, including tiny windows.
	bodyHeight := max(0, m.height-2)
	if len(lines) > bodyHeight {
		lines = lines[:bodyHeight]
	}
	for len(lines) < bodyHeight {
		lines = append(lines, "")
	}
	if m.height >= 2 {
		lines = append(lines, footer)
	}
	keys := "j/k tab pane enter logs r run p pause a approve c cancel / filter ? help q quit"
	if m.width < 80 {
		keys = "j/k ↑↓ tab enter r run p pause a approve c cancel / ? q quit"
	}
	if m.logID != "" {
		keys = "tab/s stdout/stderr  esc back  ? help  q quit"
	}
	lines = append(lines, keys)
	for i, s := range lines {
		s = strings.ReplaceAll(plain(s), "\n", " ")
		lines[i] = lipgloss.NewStyle().MaxWidth(m.width).Render(s)
	}
	return strings.Join(lines, "\n")
}
func window(total, selected, room int) (int, int) {
	start := max(0, selected-room+1)
	return start, min(total, start+max(0, room))
}
func marker(selected bool) string {
	if selected {
		return "> "
	}
	return "  "
}
func stamp(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return t.Format("01-02 15:04")
}
func jobRow(j app.JobView, width int) string {
	schedule, next := "—", "—"
	if j.Job != nil {
		schedule = j.Job.Schedule
	}
	if j.State != nil {
		next = stamp(j.State.NextFire)
	}
	last := j.LastState
	if last == "" {
		last = "—"
	}
	// All five fields remain visible at the required 60-column width.
	nameWidth := max(4, min(20, (width-48)/2))
	return fmt.Sprintf("%s | %s | %s | %s | %s", clip(j.Name, nameWidth), clip(schedule, 8), next, clip(last, 8), j.Status)
}
func clip(s string, width int) string {
	return lipgloss.NewStyle().MaxWidth(width).Render(strings.ReplaceAll(plain(s), "\n", " "))
}
func (m model) runRow(r app.Run) string {
	duration := "—"
	if r.StartedAt != nil {
		end := m.clock.Now()
		if r.EndedAt != nil {
			end = *r.EndedAt
		}
		duration = end.Sub(*r.StartedAt).Truncate(time.Second).String()
	}
	return fmt.Sprintf("%s | %s | %s | %s | %s", runID(r.ID, max(8, m.width-52)), clip(r.Trigger, 8), clip(r.State, 11), stamp(r.StartedAt), clip(duration, 8))
}

// Keep the random suffix visible when a narrow terminal abbreviates run IDs.
func runID(id string, width int) string {
	if len(id) <= width {
		return id
	}
	return "…" + id[len(id)-width+1:]
}
