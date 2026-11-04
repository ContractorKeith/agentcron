package app

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/runner"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func TestUnapprovedNeverFires(t *testing.T) {
	opts := isolated(t)
	c := clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
	if err := Init(t.Context(), opts); err != nil {
		t.Fatal(err)
	}
	j := NewJob("agent")
	j.Schedule = "every 1m"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hi"
	if _, err := CreateMCPJob(t.Context(), opts, c, j); err != nil {
		t.Fatal(err)
	}
	// Removing pause must not bypass approval, even when a fire becomes overdue.
	if err := PauseJob(t.Context(), opts, c, j.Name, false); err != nil {
		t.Fatal(err)
	}
	c.Advance(5 * time.Minute)
	if err := Schedule(t.Context(), opts, c, false, false, nil, io.Discard); err != nil {
		t.Fatal(err)
	}
	runs, err := Runs(t.Context(), opts, c, j.Name, "", 50)
	if err != nil || len(runs) != 0 {
		t.Fatal(runs, err)
	}
	if _, err := EnqueueMCPRun(t.Context(), opts, c, j.Name); Code(err) != Refused {
		t.Fatal(err)
	}
	if err := ApproveJob(t.Context(), opts, c, "missing"); Code(err) != Usage {
		t.Fatal(err)
	}
	opts.ReadOnly = true
	if err := ApproveJob(t.Context(), opts, c, j.Name); Code(err) != Usage {
		t.Fatal(err)
	}
}

func TestDoctorTrustWarning(t *testing.T) {
	opts := isolated(t)
	if err := Init(t.Context(), opts); err != nil {
		t.Fatal(err)
	}
	c := clock.Real{}
	j := NewJob("unsafe")
	j.Schedule = "@daily"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hi"
	j.Repo = filepath.Dir(opts.ConfigDir)
	if _, err := AddJob(t.Context(), opts, c, j, false); err != nil {
		t.Fatal(err)
	}
	health, err := Doctor(t.Context(), opts)
	if err != nil || len(health.Warnings) != 1 || !strings.Contains(health.Warnings[0], "unsafe") {
		t.Fatal(health, err)
	}
	// A symlinked repository must produce the same warning.
	link := filepath.Join(t.TempDir(), "repo")
	if err := os.Symlink(j.Repo, link); err != nil {
		t.Fatal(err)
	}
	j.Name = "linked"
	j.Repo = link
	if _, err := AddJob(t.Context(), opts, c, j, false); err != nil {
		t.Fatal(err)
	}
	health, err = Doctor(t.Context(), opts)
	if err != nil || len(health.Warnings) != 2 {
		t.Fatal(health, err)
	}
}

func TestConfigInsideWorktreeBlocked(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		root := t.TempDir()
		data := filepath.Join(root, "data")
		c := clock.Real{}
		r, err := store.NewRun("unsafe", "manual", c.Now())
		if err != nil {
			t.Fatal(err)
		}
		configDir := filepath.Join(data, "worktrees", "unsafe", r.ID, "config")
		opts := Options{ConfigDir: configDir, DataDir: data}
		if err := Init(t.Context(), opts); err != nil {
			t.Fatal(err)
		}
		if symlink {
			link := filepath.Join(root, "link")
			if err := os.Symlink(data, link); err != nil {
				t.Fatal(err)
			}
			opts.DataDir = link
		}
		j := NewJob("unsafe")
		j.Schedule = "@daily"
		j.Runner = "command"
		j.Command = []string{"cat"}
		j.Prompt = "hi"
		if _, err := AddJob(t.Context(), opts, c, j, false); err != nil {
			t.Fatal(err)
		}
		db, err := store.Open(t.Context(), filepath.Join(data, "agentcron.db"))
		if err != nil {
			t.Fatal(err)
		}
		if err := db.CreateRun(t.Context(), r); err != nil {
			db.Close()
			t.Fatal(err)
		}
		x := runner.Runner{DB: db, Clock: c, DataDir: opts.DataDir, JobsDir: filepath.Join(configDir, "jobs"), ConfigDir: configDir}
		got, err := x.Run(t.Context(), j, r, true)
		db.Close()
		if err != nil || got.State != "blocked_config" || !strings.Contains(got.Error, "config directory") || got.PID != nil {
			t.Fatal(got, err)
		}
		if _, err := os.Stat(filepath.Join(data, "runs", r.ID)); !os.IsNotExist(err) {
			t.Fatal("run launched", err)
		}
	}
}
