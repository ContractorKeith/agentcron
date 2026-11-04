package tui

import (
	"context"
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/ContractorKeith/agentcron/internal/app"
)

func TestRefreshErrorsAndStaleResults(t *testing.T) {
	m, f := fixture()
	f.err = errors.New("ledger unavailable")
	m = deliver(m, m.load())
	if !strings.Contains(m.render(), "ledger unavailable") {
		t.Fatal(m.render())
	}
	old := m.load()()
	m.generation++
	next, _ := m.Update(old)
	if next.(model).generation != m.generation {
		t.Fatal("stale refresh applied")
	}
	f.err = nil
	m, cmd := press(m, "r")
	f.err = errors.New("write failed")
	m = deliver(m, cmd)
	if m.footer != "write failed" {
		t.Fatal(m.footer)
	}
}
func TestQuitAndNoGoroutineLeak(t *testing.T) {
	before := runtime.NumGoroutine()
	for range 100 {
		m, _ := fixture()
		for _, key := range []string{"q", "ctrl+c"} {
			_, cmd := press(m, key)
			if cmd == nil || cmd() != tea.Quit() {
				t.Fatal("not tea.Quit")
			}
		}
		if !m.View().AltScreen {
			t.Fatal("must use managed alternate screen")
		}
	}
	runtime.GC()
	if after := runtime.NumGoroutine(); after > before {
		t.Fatalf("goroutines before %d after %d", before, after)
	}
}

func TestProgramShutdownWithoutTerminal(t *testing.T) {
	before := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m, _ := fixture()
	m.ctx = ctx
	// Explicit streams and the nil renderer never open a real terminal.
	_, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(strings.NewReader("q")), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler(), tea.WithEnvironment([]string{"TERM=dumb"})).Run()
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for runtime.NumGoroutine() > before {
		select {
		case <-deadline.C:
			t.Fatalf("goroutines before %d after %d", before, runtime.NumGoroutine())
		case <-time.After(time.Millisecond):
			runtime.Gosched()
		}
	}
}

func TestTickRefreshAndSelectionIdentity(t *testing.T) {
	m, f := fixture()
	next, cmd := m.Update(tickMsg{})
	m = next.(model)
	if cmd == nil || !m.loading {
		t.Fatal("tick did not start refresh")
	}
	batch := cmd().(tea.BatchMsg)
	m = deliver(m, batch[0])
	if m.loading {
		t.Fatal("refresh stayed pending")
	}
	f.runs = append([]app.Run{{ID: "r_new", State: "queued"}}, f.runs...)
	m = deliver(m, m.load())
	if m.runs[m.runIndex].ID != "r_one" {
		t.Fatal("refresh moved run selection")
	}
}
