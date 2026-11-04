package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		xdg, file, env, flags bool
		want                  string
	}{
		{name: "home fallback", want: "home"},
		{name: "XDG", xdg: true, want: "xdg"},
		{name: "file over defaults", xdg: true, file: true, want: "file"},
		{name: "env over file", xdg: true, file: true, env: true, want: "env"},
		{name: "flags over env and file", xdg: true, file: true, env: true, flags: true, want: "flags"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scratch := t.TempDir()
			t.Setenv("HOME", filepath.Join(scratch, "home"))
			t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(scratch, "gitconfig"))
			for _, key := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "AGENTCRON_CONFIG_DIR", "AGENTCRON_DATA_DIR"} {
				t.Setenv(key, "")
			}
			paths := map[string]Options{
				"home": {filepath.Join(scratch, "home", ".config", "agentcron"), filepath.Join(scratch, "home", ".local", "share", "agentcron")},
			}
			for _, key := range []string{"xdg", "file", "env", "flags"} {
				paths[key] = Options{filepath.Join(scratch, key, "agentcron"), filepath.Join(scratch, key, "data")}
			}
			if tc.xdg {
				t.Setenv("XDG_CONFIG_HOME", filepath.Join(scratch, "xdg"))
				t.Setenv("XDG_DATA_HOME", filepath.Join(scratch, "xdg-data"))
				paths["xdg"] = Options{paths["xdg"].ConfigDir, filepath.Join(scratch, "xdg-data", "agentcron")}
			}
			if tc.env {
				t.Setenv("AGENTCRON_CONFIG_DIR", paths["env"].ConfigDir)
				t.Setenv("AGENTCRON_DATA_DIR", paths["env"].DataDir)
			}
			var opts Options
			if tc.flags {
				opts = paths["flags"]
			}
			if tc.file {
				source := paths["xdg"].ConfigDir
				if tc.env {
					source = paths["env"].ConfigDir
				}
				if tc.flags {
					source = paths["flags"].ConfigDir
				}
				if err := os.MkdirAll(source, 0o700); err != nil {
					t.Fatal(err)
				}
				contents := "data_dir = '" + paths["file"].DataDir + "'\nconcurrency = 4\nkeep_runs = 7\nallow_agent_scheduling = true\nmdkb_root = 'scratch-kb'\n"
				if err := os.WriteFile(filepath.Join(source, "config.toml"), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := Load(opts)
			if err != nil {
				t.Fatal(err)
			}
			want := paths[tc.want]
			if tc.want == "file" {
				want.ConfigDir = paths["xdg"].ConfigDir
			}
			if got.ConfigDir != want.ConfigDir || got.DataDir != want.DataDir {
				t.Fatalf("paths = %s, %s; want %+v", got.ConfigDir, got.DataDir, want)
			}
			if tc.file {
				if got.Concurrency != 4 || got.KeepRuns != 7 || !got.AllowAgentScheduling || got.MDKBRoot != "scratch-kb" {
					t.Fatalf("file settings: %+v", got)
				}
			} else if got.Concurrency != 2 || got.KeepRuns != 50 || got.AllowAgentScheduling || got.MDKBRoot != "" {
				t.Fatalf("defaults: %+v", got)
			}
		})
	}
}

func TestInvalidConfig(t *testing.T) {
	scratch := t.TempDir()
	t.Setenv("HOME", scratch)
	t.Setenv("XDG_CONFIG_HOME", scratch)
	t.Setenv("XDG_DATA_HOME", scratch)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(scratch, "gitconfig"))
	t.Setenv("AGENTCRON_CONFIG_DIR", "")
	t.Setenv("AGENTCRON_DATA_DIR", "")
	for _, contents := range []string{"concurrency = 0", "keep_runs = -1", "concurrency = ["} {
		if err := os.WriteFile(filepath.Join(scratch, "config.toml"), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(Options{ConfigDir: scratch}); err == nil {
			t.Fatalf("accepted %q", contents)
		}
	}
}
