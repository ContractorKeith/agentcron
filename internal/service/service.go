// Package service renders and manages systemd user and launchd service definitions.
package service

import (
	"context"
	"fmt"
	"github.com/ContractorKeith/agentcron/internal/clock"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Commander interface {
	Run(context.Context, string, ...string) (string, error)
}
type Exec struct{}

func (Exec) Run(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s: %w: %s", name, err, out)
	}
	return string(out), nil
}

type Options struct {
	OS, Home, Binary, Path, ConfigDir, DataDir string
	MDKBRoot                                   string
	UID                                        int
	Clock                                      clock.Clock
}
type Definition struct {
	Path        string            `json:"path"`
	Unit        string            `json:"unit"`
	Commands    [][]string        `json:"commands"`
	Environment map[string]string `json:"environment"`
	Warnings    []string          `json:"warnings,omitempty"`
}
type State struct {
	Installed bool `json:"installed"`
	Active    bool `json:"active"`
}

func Current(binary, path, config, data string) (Options, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Options{}, err
	}
	binary, err = filepath.Abs(binary)
	return Options{OS: runtime.GOOS, Home: home, Binary: binary, Path: path, ConfigDir: config, DataDir: data, UID: os.Getuid(), MDKBRoot: os.Getenv("MDKB_ROOT")}, err
}
func (o Options) domain() string { return "gui/" + strconv.Itoa(o.UID) }
func (o Options) file() string {
	if o.OS == "darwin" {
		return filepath.Join(o.Home, "Library", "LaunchAgents", "com.contractorkeith.agentcron.plist")
	}
	return filepath.Join(o.Home, ".config", "systemd", "user", "agentcron.service")
}
func (o Options) args() []string {
	args := []string{o.Binary}
	if o.ConfigDir != "" {
		args = append(args, "--config", o.ConfigDir)
	}
	if o.DataDir != "" {
		args = append(args, "--data-dir", o.DataDir)
	}
	return append(args, "daemon")
}
func (o Options) Commands(action string) [][]string {
	if o.OS == "darwin" {
		if action == "install" {
			return [][]string{{"launchctl", "bootout", o.domain() + "/com.contractorkeith.agentcron"}, {"launchctl", "print", o.domain() + "/com.contractorkeith.agentcron"}, {"launchctl", "bootstrap", o.domain(), o.file()}}
		}
		return [][]string{{"launchctl", "bootout", o.domain() + "/com.contractorkeith.agentcron"}}
	}
	if action == "install" {
		return [][]string{{"systemctl", "--user", "daemon-reload"}, {"systemctl", "--user", "enable", "agentcron.service"}, {"systemctl", "--user", "restart", "agentcron.service"}}
	}
	return [][]string{{"systemctl", "--user", "disable", "--now", "agentcron.service"}, {"systemctl", "--user", "daemon-reload"}}
}
func Apply(ctx context.Context, o Options, action string, dry bool, cmd Commander) (Definition, error) {
	d, err := Render(o)
	if err != nil {
		return d, err
	}
	d.Commands = o.Commands(action)
	if dry {
		return d, nil
	}
	if action == "install" && o.OS == "darwin" {
		return installLaunchd(ctx, o, d, cmd)
	}
	if action == "install" {
		if err = os.MkdirAll(filepath.Dir(d.Path), 0700); err != nil {
			return d, err
		}
		if err = os.MkdirAll(o.DataDir, 0700); err != nil {
			return d, err
		}
		if err = os.WriteFile(d.Path, []byte(d.Unit), 0600); err != nil {
			return d, err
		}
	}
	for i, c := range d.Commands {
		// Remove the unit before the final systemd reload.
		if action == "uninstall" && i == 1 {
			if err = remove(d.Path); err != nil {
				return d, err
			}
		}
		out, runErr := cmd.Run(ctx, c[0], c[1:]...)
		if runErr != nil {
			if action == "uninstall" && o.OS == "linux" {
				d.Warnings = append(d.Warnings, runErr.Error()+": "+strings.TrimSpace(out))
				continue
			}
			if o.OS == "darwin" && c[1] == "bootout" && notLoaded(out+" "+runErr.Error()) {
				continue
			}
			return d, runErr
		}
	}
	if action == "uninstall" {
		err = remove(d.Path)
	}
	return d, err
}
func remove(path string) error {
	e := os.Remove(path)
	if os.IsNotExist(e) {
		return nil
	}
	return e
}
func Status(ctx context.Context, o Options, cmd Commander) (State, error) {
	_, err := os.Stat(o.file())
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	s := State{Installed: true}
	if o.OS == "darwin" {
		out, err := cmd.Run(ctx, "launchctl", "print", o.domain()+"/com.contractorkeith.agentcron")
		s.Active = err == nil && strings.Contains(out, "state = running")
	} else {
		out, e := cmd.Run(ctx, "systemctl", "--user", "is-active", "agentcron.service")
		s.Active = e == nil && strings.TrimSpace(out) == "active"
	}
	return s, nil
}

func notLoaded(s string) bool {
	s = strings.ToLower(s)
	return strings.Contains(s, "no such process") || strings.Contains(s, "could not find service") || strings.Contains(s, "not found") || strings.Contains(s, "not loaded")
}
