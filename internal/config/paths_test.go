package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAbsolutePaths(t *testing.T) {
	scratch := t.TempDir()
	t.Setenv("HOME", scratch)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(scratch, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(scratch, "data"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(scratch, "gitconfig"))
	t.Setenv("AGENTCRON_CONFIG_DIR", "")
	t.Setenv("AGENTCRON_DATA_DIR", "")
	t.Chdir(scratch)
	t.Setenv("PWD", scratch)
	for _, tc := range []struct {
		name, configDir, dataDir, fileData string
		env                                bool
		wantConfig, wantData               string
	}{
		{name: "relative flags", configDir: "relative-config", dataDir: "relative-data", wantConfig: "relative-config", wantData: "relative-data"},
		{name: "home flags", configDir: "~/flag-config", dataDir: "~/flag-data", wantConfig: "flag-config", wantData: "flag-data"},
		{name: "relative env", configDir: "env-config", dataDir: "env-data", env: true, wantConfig: "env-config", wantData: "env-data"},
		{name: "home env", configDir: "~/env-config", dataDir: "~/env-data", env: true, wantConfig: "env-config", wantData: "env-data"},
		{name: "home in file", configDir: "~/file-config", fileData: "~/x", wantConfig: "file-config", wantData: "x"},
		{name: "relative in file", configDir: "file-config", fileData: "relative-file-data", wantConfig: "file-config", wantData: "relative-file-data"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := filepath.Join(scratch, tc.wantConfig)
			if err := os.MkdirAll(source, 0o700); err != nil {
				t.Fatal(err)
			}
			// config_dir must never redirect the directory containing the settings and jobs.
			contents := "config_dir = 'ignored'\n"
			if tc.fileData != "" {
				contents += "data_dir = '" + tc.fileData + "'\n"
			}
			if err := os.WriteFile(filepath.Join(source, "config.toml"), []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			opts := Options{ConfigDir: tc.configDir, DataDir: tc.dataDir}
			if tc.env {
				t.Setenv("AGENTCRON_CONFIG_DIR", tc.configDir)
				t.Setenv("AGENTCRON_DATA_DIR", tc.dataDir)
				opts = Options{}
			}
			got, err := Load(opts)
			if err != nil {
				t.Fatal(err)
			}
			if got.ConfigDir != source || got.DataDir != filepath.Join(scratch, tc.wantData) {
				t.Fatalf("paths = %q, %q", got.ConfigDir, got.DataDir)
			}
		})
	}
}

func TestFileCannotRedirectConfigDirectory(t *testing.T) {
	scratch := t.TempDir()
	t.Setenv("HOME", scratch)
	t.Setenv("XDG_CONFIG_HOME", scratch)
	t.Setenv("XDG_DATA_HOME", scratch)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(scratch, "gitconfig"))
	t.Setenv("AGENTCRON_CONFIG_DIR", "")
	t.Setenv("AGENTCRON_DATA_DIR", "")
	source := filepath.Join(scratch, "agentcron")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "config.toml"), []byte("config_dir = 'split-state'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got.ConfigDir != source {
		t.Fatalf("file redirected config directory to %q", got.ConfigDir)
	}
}
