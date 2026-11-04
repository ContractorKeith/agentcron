package jobs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestExactNamesThroughJobAPIs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "morning.toml")
	// The lowercase path is readable on this host, but APFS reports the real
	// directory entry as Morning.toml. Every API must honor that listing.
	raw := []byte("schedule = '@daily'\nrunner = 'command'\ncommand = ['cat']\nprompt = 'hi'\n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	original := readJobDir
	t.Cleanup(func() { readJobDir = original })
	readJobDir = func(string) ([]os.DirEntry, error) { return []os.DirEntry{fakeEntry{name: "Morning.toml"}}, nil }
	exists, err := Exists(dir, "morning")
	if err != nil || exists {
		t.Errorf("exists=%v err=%v", exists, err)
	}
	for _, read := range []func(string, string) (Job, error){Read, ReadForRun} {
		if _, err := read(dir, "morning"); !errors.Is(err, os.ErrNotExist) {
			t.Error(err)
		}
	}
	if err := Remove(dir, "morning"); !errors.Is(err, os.ErrNotExist) {
		t.Error(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(raw) {
		t.Fatal("case variant modified", err)
	}
}
