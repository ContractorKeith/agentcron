package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/agentcron/internal/clock"
)

func TestTailOutputBounds(t *testing.T) {
	opts := isolated(t)
	c := clock.Real{}
	if err := Init(t.Context(), opts); err != nil {
		t.Fatal(err)
	}
	j := NewJob("output")
	j.Schedule = "@daily"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hi"
	if _, err := AddJob(t.Context(), opts, c, j, false); err != nil {
		t.Fatal(err)
	}
	r, err := RunJob(t.Context(), opts, c, j.Name, false, false)
	if err != nil {
		t.Fatal(err)
	}
	out, err := TailOutput(t.Context(), opts, c, r.ID, "stdout", 16384)
	if err != nil || out.Output != "" || out.Truncated {
		t.Fatal(out, err)
	}
	dir := filepath.Join(opts.DataDir, "runs", r.ID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stdout.log"), []byte(strings.Repeat("a", 300000)+"END"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{1, 16384, 262144} {
		out, err := TailOutput(t.Context(), opts, c, r.ID, "stdout", n)
		if err != nil || len(out.Output) != n || !out.Truncated || !strings.HasSuffix(out.Output, "D") {
			t.Fatal(out, err)
		}
	}
	for _, n := range []int{0, -1, 262145} {
		if _, err := TailOutput(t.Context(), opts, c, r.ID, "stdout", n); err == nil {
			t.Fatal("accepted limit", n)
		}
	}
	if _, err := TailOutput(t.Context(), opts, c, "missing", "stdout", 1); err == nil {
		t.Fatal("accepted missing run")
	}
}
