package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/app"
)

func isolate(t *testing.T) string {
	t.Helper()
	// Keep compiler caches available after moving HOME into the scratch fixture.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("GOPATH") == "" {
		t.Setenv("GOPATH", filepath.Join(home, "go"))
	}
	if os.Getenv("GOCACHE") == "" {
		t.Setenv("GOCACHE", filepath.Join(cache, "go-build"))
	}
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, "gitconfig"))
	t.Setenv("AGENTCRON_CONFIG_DIR", "")
	t.Setenv("AGENTCRON_DATA_DIR", "")
	return dir
}

func build(t *testing.T, dest, pkg string, flags ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	args := append([]string{"build", "-o", dest}, flags...)
	args = append(args, pkg)
	cmd := testCommand(ctx, "go", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %s %v", out, err)
	}
}

func TestVersionBuilds(t *testing.T) {
	scratch := isolate(t)
	for _, value := range []string{"dev", "v0.0.0-test"} {
		t.Run(value, func(t *testing.T) {
			bin := filepath.Join(scratch, "agentcron-"+value)
			var flags []string
			if value != "dev" {
				flags = []string{"-ldflags", "-X main.version=" + value}
			}
			build(t, bin, ".", flags...)
			cmd := testCommand(context.Background(), bin, "version")
			out, err := cmd.CombinedOutput()
			if err != nil || string(out) != value+"\n" {
				t.Fatalf("version: %q %v", out, err)
			}
		})
	}
}

func TestOutputAndUsage(t *testing.T) {
	isolate(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"version", "--json"}, "{\"version\":\"dev\"}\n"},
		{[]string{"--quiet", "version"}, ""},
		{[]string{"--quiet", "version", "--json"}, "{\"version\":\"dev\"}\n"},
	} {
		root := newRoot()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs(tc.args)
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if out.String() != tc.want {
			t.Fatalf("%v = %q", tc.args, out.String())
		}
	}
	for _, args := range [][]string{{"nonesuch"}, {"version", "extra"}, {"--bad-flag"}} {
		if err := execute(args); app.Code(err) != app.Usage {
			t.Fatalf("%v exit %d: %v", args, app.Code(err), err)
		}
	}
}

// Kill the group on deadline so compiler or fake descendants cannot escape cleanup.
func testCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 5 * time.Second
	return cmd
}
