package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func setup(t *testing.T) (Runner, jobs.Job) {
	t.Helper()
	dir := t.TempDir()
	for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(key, dir)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	if e := os.WriteFile(os.Getenv("GIT_CONFIG_GLOBAL"), []byte("[user]\n name = Test\n email = test@example.com\n"), 0600); e != nil {
		t.Fatal(e)
	}
	bin := filepath.Join(dir, "bin")
	if e := os.Mkdir(bin, 0700); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"claude", "codex"} {
		cmd := exec.Command("go", "build", "-o", filepath.Join(bin, name), "../../testdata/fakes/"+name)
		// HOME is isolated; retain the build cache supplied by the test harness.
		cmd.Env = append(os.Environ(), "GOCACHE="+testCache)
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("build: %s %v", out, e)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, name := range []string{"claude", "codex"} {
		p, e := exec.LookPath(name)
		if e != nil || p != filepath.Join(bin, name) {
			t.Fatal("fake not first on PATH")
		}
	}
	for _, key := range []string{"FAKE_MODE", "FAKE_RECORD", "FAKE_STDIN", "FAKE_NOISY_STREAM"} {
		t.Setenv(key, "")
	}
	t.Setenv("FAKE_SLEEP", "2s")
	db, e := store.Open(context.Background(), filepath.Join(dir, "db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	j := jobs.Defaults("fixture")
	j.Schedule = "@daily"
	j.Runner = "claude"
	j.Prompt = "hello"
	j.Timeout = "5s"
	return Runner{DB: db, Clock: clock.Real{}, DataDir: dir, JobsDir: dir, Grace: 200 * time.Millisecond}, j
}

var testCache = os.Getenv("GOCACHE")

func init() {
	if testCache == "" {
		testCache = filepath.Join(os.Getenv("HOME"), ".cache", "go-build")
	}
}
func executeRun(t *testing.T, x Runner, j jobs.Job, approved bool) store.Run {
	t.Helper()
	r, e := store.NewRun(j.Name, "manual", x.Clock.Now())
	if e != nil {
		t.Fatal(e)
	}
	if e = x.DB.CreateRun(context.Background(), r); e != nil {
		t.Fatal(e)
	}
	r, e = x.Run(context.Background(), j, r, approved)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func repo(t *testing.T, x Runner) string {
	t.Helper()
	path := filepath.Join(x.DataDir, "repo")
	if e := os.Mkdir(path, 0700); e != nil {
		t.Fatal(e)
	}
	for _, args := range [][]string{{"init"}, {"commit", "--allow-empty", "-m", "initial"}} {
		if _, e := git(context.Background(), path, args...); e != nil {
			t.Fatal(e)
		}
	}
	return path
}
