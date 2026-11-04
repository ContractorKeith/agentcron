package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/ContractorKeith/agentcron/internal/app"
)

func TestViewSnapshots(t *testing.T) {
	for _, name := range []string{"empty", "narrow", "scroll", "logs"} {
		t.Run(name, func(t *testing.T) {
			m, _ := fixture()
			switch name {
			case "empty":
				m.jobs = nil
				m.runs = nil
			case "narrow":
				m.width = 60
			case "scroll":
				for i := range 50 {
					m.jobs = append(m.jobs, app.JobView{Name: fmt.Sprintf("job-%02d", i), Status: "paused"})
				}
				for range 45 {
					m, _ = press(m, "j")
				}
				if !strings.Contains(m.render(), "> job-44") {
					t.Fatal("selection outside viewport")
				}
			case "logs":
				m.logID = "r_one"
				m.output = "first\nsecond\n"
			}
			got := m.render() + "\n"
			path := filepath.Join("testdata", name+".golden")
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.WriteFile(path, []byte(got), 0600); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Fatalf("snapshot mismatch:\n%s", got)
			}
			for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
				if lipgloss.Width(line) > m.width {
					t.Fatalf("line wider than %d: %q", m.width, line)
				}
			}
		})
	}
}
func TestTinyAndHostileOutput(t *testing.T) {
	m, _ := fixture()
	m.logID = "r_one"
	m.output = strings.Repeat("line\n", 20000) + "\x1b]52;secret\a\x1b[2J"
	for _, size := range [][2]int{{1, 1}, {20, 2}, {60, 10}} {
		m.width, m.height = size[0], size[1]
		view := m.render()
		if strings.Count(view, "\n")+1 > m.height || strings.Contains(view, "\x1b") {
			t.Fatal("unsafe or oversized frame")
		}
	}
}

func TestNarrowRunIDsRemainDistinct(t *testing.T) {
	first := runID("r_20261104T120000Z_ABCDEFGH", 8)
	second := runID("r_20261104T120000Z_IJKLMNOP", 8)
	if first == second || lipgloss.Width(first) > 8 {
		t.Fatalf("ambiguous IDs: %q %q", first, second)
	}
}
