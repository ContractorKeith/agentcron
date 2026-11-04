// Package testutil builds and warms fake runners before process timing tests.
package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Fakes returns an isolated directory and cleanup; call before testing.M.Run.
func Fakes() (string, func(), error) {
	dir, err := os.MkdirTemp("", "agentcron-fakes-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	for _, name := range []string{"claude", "codex", "mdkb"} {
		binary := filepath.Join(dir, name)
		if out, err := exec.Command("go", "build", "-o", binary, "../../testdata/fakes/"+name).CombinedOutput(); err != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("build %s: %w: %s", name, err, out)
		}
		cmd := exec.Command(binary, "version")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "HOME="+dir, "MDKB_ROOT="+filepath.Join(dir, "kb"), "FAKE_MODE=ok", "FAKE_RECORD=", "FAKE_STDIN=", "FAKE_SLEEP=", "FAKE_MDKB_CAT_SLEEP=", "FAKE_MDKB_NEW_SLEEP=")
		if out, err := cmd.CombinedOutput(); err != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("warm %s: %w: %s", name, err, out)
		}
	}
	return dir, cleanup, nil
}

// ValidateFakes checks inherited fixture paths before any subprocess can use them.
func ValidateFakes(dir string) error {
	for _, name := range []string{"claude", "codex", "mdkb"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("invalid inherited fakes: %s: %w", name, err)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
			return fmt.Errorf("invalid inherited fakes: %s is not an executable regular file", name)
		}
	}
	return nil
}
