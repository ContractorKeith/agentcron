// Package config resolves settings and XDG paths on Linux and macOS.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const Defaults = "concurrency = 2\nkeep_runs = 50\nallow_agent_scheduling = false\nmdkb_root = \"\"\n"

type Options struct {
	ConfigDir string
	DataDir   string
}

type Config struct {
	ConfigDir            string `toml:"config_dir"`
	DataDir              string `toml:"data_dir"`
	Concurrency          int    `toml:"concurrency"`
	KeepRuns             int    `toml:"keep_runs"`
	AllowAgentScheduling bool   `toml:"allow_agent_scheduling"`
	MDKBRoot             string `toml:"mdkb_root"`
}

func (c Config) File() string     { return filepath.Join(c.ConfigDir, "config.toml") }
func (c Config) JobsDir() string  { return filepath.Join(c.ConfigDir, "jobs") }
func (c Config) Database() string { return filepath.Join(c.DataDir, "agentcron.db") }

// Load finds config.toml using flag > env > XDG, then overlays paths in the same order.
// A config_dir in that file selects the effective directory; it does not load a second file.
func Load(opts Options) (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}
	configHome := first(os.Getenv("XDG_CONFIG_HOME"), filepath.Join(home, ".config"))
	dataHome := first(os.Getenv("XDG_DATA_HOME"), filepath.Join(home, ".local", "share"))
	c := Config{
		ConfigDir:   filepath.Join(configHome, "agentcron"),
		DataDir:     filepath.Join(dataHome, "agentcron"),
		Concurrency: 2, KeepRuns: 50,
	}
	source := first(opts.ConfigDir, os.Getenv("AGENTCRON_CONFIG_DIR"), c.ConfigDir)
	if _, err := toml.DecodeFile(filepath.Join(source, "config.toml"), &c); err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	c.ConfigDir = first(opts.ConfigDir, os.Getenv("AGENTCRON_CONFIG_DIR"), c.ConfigDir)
	c.DataDir = first(opts.DataDir, os.Getenv("AGENTCRON_DATA_DIR"), c.DataDir)
	if c.Concurrency < 1 || c.KeepRuns < 0 {
		return Config{}, fmt.Errorf("concurrency must be positive and keep_runs nonnegative")
	}
	return c, nil
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
