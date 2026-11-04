// Package memory owns optional mdkb discovery and note access.
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
)

type Client struct{ Binary, Root string }
type Health struct {
	Present       bool
	Version, Root string
	Initialised   bool
}

// Resolve honors agentcron settings, then PATH and MDKB_ROOT. An unset root is
// deliberately left to mdkb, including its own configuration file.
func Resolve(configFile string) (Client, error) {
	var settings struct {
		Path string `toml:"mdkb_path"`
		Root string `toml:"mdkb_root"`
	}
	if _, err := toml.DecodeFile(configFile, &settings); err != nil && !os.IsNotExist(err) {
		return Client{}, err
	}
	c := Client{Root: settings.Root}
	if c.Root == "" {
		c.Root = os.Getenv("MDKB_ROOT")
	}
	if c.Root == "~" || strings.HasPrefix(c.Root, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return c, err
		}
		c.Root = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(c.Root, "~"), "/"))
	}
	if c.Root != "" {
		var err error
		c.Root, err = filepath.Abs(c.Root)
		if err != nil {
			return c, err
		}
	}
	name := settings.Path
	if name == "" {
		name = "mdkb"
	}
	if strings.HasPrefix(name, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return c, err
		}
		name = filepath.Join(home, name[2:])
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return c, fmt.Errorf("mdkb not available: %w", err)
	}
	c.Binary, err = filepath.Abs(path)
	return c, err
}

func (c Client) Args(args ...string) []string {
	if c.Root != "" {
		return append([]string{"--root", c.Root}, args...)
	}
	return args
}

// Run bounds helper processes and kills their whole group on cancellation.
func (c Client) Run(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Binary, c.Args(args...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("mdkb %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func (c Client) Status(ctx context.Context) Health {
	h := Health{Present: c.Binary != "", Root: c.Root}
	if !h.Present {
		return h
	}
	if out, err := c.Run(ctx, "version"); err == nil {
		h.Version = strings.TrimSpace(string(out))
	}
	out, err := c.Run(ctx, "info", "--json")
	if err == nil {
		var info struct {
			Root        string `json:"root"`
			Initialised bool   `json:"initialized"`
		}
		if json.Unmarshal(out, &info) == nil {
			h.Initialised = info.Initialised
			if info.Root != "" {
				h.Root = info.Root
			}
		}
	}
	return h
}
