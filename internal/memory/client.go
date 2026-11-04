// Package memory owns optional mdkb discovery and note access.
package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type Client struct{ Binary, Root string }
type Health struct {
	Present       bool
	Version, Root string
	Initialised   bool
}

// Resolve finds the executable using resolved configuration values or PATH.
func Resolve(binary, root string) (Client, error) {
	c := Client{Root: root}
	if binary == "" {
		binary = "mdkb"
	}
	path, err := exec.LookPath(binary)
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
	limit := 5 * time.Second
	if len(args) > 0 && args[0] == "new" {
		limit = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Binary, c.Args(args...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if err != nil {
		detail := ""
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			stderr := []byte(strings.TrimSpace(string(exit.Stderr)))
			if len(stderr) > 500 {
				stderr = stderr[:500]
			}
			if len(stderr) > 0 {
				detail = ": " + string(stderr)
			}
		}
		root := c.Root
		if root == "" {
			root = "mdkb default"
		}
		return nil, fmt.Errorf("mdkb %s (root %s): %w%s", strings.Join(args, " "), root, err, detail)
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
