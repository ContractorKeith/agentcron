package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func isolated(t *testing.T) Options {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, "gitconfig"))
	t.Setenv("AGENTCRON_CONFIG_DIR", "")
	t.Setenv("AGENTCRON_DATA_DIR", "")
	return Options{ConfigDir: filepath.Join(dir, "config", "agentcron"), DataDir: filepath.Join(dir, "data", "agentcron")}
}

func TestInitAndDoctor(t *testing.T) {
	opts := isolated(t)
	ctx := context.Background()
	before, err := Doctor(ctx, opts)
	if Code(err) != NotInitialized || before.ConfigDir || before.DataDir {
		t.Fatalf("before init: %+v %v", before, err)
	}
	if _, err := os.Stat(opts.DataDir); !os.IsNotExist(err) {
		t.Fatalf("doctor created data directory: %v", err)
	}
	if err := Init(ctx, opts); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(opts.ConfigDir, "config.toml")
	custom := []byte("concurrency = 3\nkeep_runs = 9\n")
	if err := os.WriteFile(configPath, custom, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Init(ctx, opts); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(configPath)
	if err != nil || string(got) != string(custom) {
		t.Fatalf("settings overwritten: %q %v", got, err)
	}
	health, err := Doctor(ctx, opts)
	if err != nil || !health.ConfigDir || !health.DataDir || health.SchemaVersion != 4 {
		t.Fatalf("health %+v: %v", health, err)
	}
	if info, err := os.Stat(filepath.Join(opts.ConfigDir, "jobs")); err != nil || !info.IsDir() {
		t.Fatalf("jobs directory: %v", err)
	}
}
