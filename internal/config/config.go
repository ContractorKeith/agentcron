// Package config resolves settings and XDG paths on Linux and macOS.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const Defaults = "concurrency = 2\nkeep_runs = 50\nallow_agent_scheduling = false\nmdkb_root = \"\"\n"

type Options struct {
	ConfigDir string
	DataDir   string
}

type Config struct {
	ConfigDir            string `toml:"-"`
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
// The config directory stays with that source; only data_dir can come from the file.
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
	c.ConfigDir, err = absolutePath(first(opts.ConfigDir, os.Getenv("AGENTCRON_CONFIG_DIR"), c.ConfigDir), home)
	if err != nil {
		return Config{}, err
	}
	defaultData := c.DataDir
	if _, err := toml.DecodeFile(c.File(), &c); err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	c.DataDir, err = absolutePath(first(opts.DataDir, os.Getenv("AGENTCRON_DATA_DIR"), c.DataDir, defaultData), home)
	if err != nil {
		return Config{}, err
	}
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

func absolutePath(path, home string) (string, error) {
	if path == "~" {
		path = home
	} else if strings.HasPrefix(path, "~/") {
		path = filepath.Join(home, path[2:])
	}
	return filepath.Abs(path)
}
