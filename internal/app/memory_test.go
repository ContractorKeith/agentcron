package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ContractorKeith/agentcron/internal/clock"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "agentcron-app-fakes-")
	if err != nil {
		panic(err)
	}
	cmd := exec.Command("go", "build", "-o", filepath.Join(dir, "mdkb"), "../../testdata/fakes/mdkb")
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic(string(out))
	}
	os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if found, err := exec.LookPath("mdkb"); err != nil || found != filepath.Join(dir, "mdkb") {
		panic("fake mdkb not first")
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestMemoryReadAndMissingBinary(t *testing.T) {
	opts := isolated(t)
	ctx := context.Background()
	if err := Init(ctx, opts); err != nil {
		t.Fatal(err)
	}
	if _, err := Memory(ctx, opts, "unknown"); Code(err) != Usage {
		t.Fatal(err)
	}
	j := NewJob("note")
	j.Schedule = "@daily"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hi"
	if _, err := AddJob(ctx, opts, clock.Real{}, j, false); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "kb")
	t.Setenv("MDKB_ROOT", root)
	if err := os.MkdirAll(filepath.Join(root, "agentcron"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".mdkb"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "agentcron", "note.md"), []byte("remember\n"), 0600); err != nil {
		t.Fatal(err)
	}
	note, err := Memory(ctx, opts, "note")
	if err != nil || note != "remember\n" {
		t.Fatalf("%q %v", note, err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err = Memory(ctx, opts, "note"); err == nil || Code(err) != Runtime {
		t.Fatal(err)
	}
}
