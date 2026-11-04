package app

import (
	"github.com/ContractorKeith/agentcron/internal/clock"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrunePreviewMatchesDecisions(t *testing.T) {
	opts := isolated(t)
	c := clock.Real{}
	if err := Init(t.Context(), opts); err != nil {
		t.Fatal(err)
	}
	j := NewJob("preview")
	j.Schedule = "@daily"
	j.Runner = "command"
	j.Command = []string{"touch", "result"}
	j.Prompt = "hi"
	j.Memory = false
	if _, err := AddJob(t.Context(), opts, c, j, false); err != nil {
		t.Fatal(err)
	}
	r, err := RunJob(t.Context(), opts, c, j.Name, true, true)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := Prune(t.Context(), opts, c, true, false)
	if err != nil || len(preview) != 1 {
		t.Fatal(preview, err)
	}
	if !strings.Contains(preview[0].Error, "kept") || !strings.Contains(preview[0].Error, "result") {
		t.Errorf("no keep decision: %+v", preview)
	}
	actual, err := Prune(t.Context(), opts, c, false, false)
	if err != nil || len(actual) != 1 || actual[0].Error != preview[0].Error {
		t.Fatal(actual, err)
	}
	forced, err := Prune(t.Context(), opts, c, true, true)
	if err != nil || !strings.Contains(forced[0].Error, "would remove") || !strings.Contains(forced[0].Error, "result") {
		t.Errorf("no force decision: %+v %v", forced, err)
	}
	if err := os.Remove(filepath.Join(r.Worktree, "result")); err != nil {
		t.Fatal(err)
	}
	preview, err = Prune(t.Context(), opts, c, true, false)
	if err != nil || !strings.Contains(preview[0].Error, "would remove") {
		t.Errorf("no remove decision: %+v %v", preview, err)
	}
	if _, err := os.Stat(r.Worktree); err != nil {
		t.Fatal("preview removed worktree", err)
	}
	if _, err := Prune(t.Context(), opts, c, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(r.Worktree); !os.IsNotExist(err) {
		t.Fatal("real prune did not remove", err)
	}
}
