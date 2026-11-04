package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type lifecycleCommander struct {
	calls   []string
	missing bool
}

func (f *lifecycleCommander) Run(_ context.Context, name string, args ...string) (string, error) {
	s := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, s)
	if strings.Contains(s, "bootout") && f.missing {
		return "Boot-out failed: 3: No such process", errors.New("not loaded")
	}
	if strings.Contains(s, "launchctl print") {
		return "Could not find service", errors.New("not found")
	}
	if strings.Contains(s, "is-active") {
		return "active", nil
	}
	return "", nil
}
func TestReinstallAndUnloadedRemoval(t *testing.T) {
	for _, target := range []string{"darwin", "linux"} {
		t.Run(target, func(t *testing.T) {
			o := Options{OS: target, Home: t.TempDir(), DataDir: t.TempDir(), Binary: "/agentcron", UID: 501}
			f := &lifecycleCommander{missing: true}
			d, err := Apply(t.Context(), o, "install", false, f)
			if err != nil {
				t.Fatal(err)
			}
			calls := strings.Join(f.calls, "\n")
			if target == "darwin" && !strings.Contains(calls, "bootout") {
				t.Error(calls)
			}
			if target == "linux" && (!strings.Contains(calls, "restart agentcron.service") || !strings.Contains(d.Unit, "KillMode=mixed\nTimeoutStopSec=30")) {
				t.Error(calls, d.Unit)
			}
			if _, err := Apply(t.Context(), o, "uninstall", false, f); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Clean(d.Path)); !os.IsNotExist(err) {
				t.Fatal(err)
			}
		})
	}
}
