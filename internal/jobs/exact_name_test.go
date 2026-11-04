package jobs

import (
	"os"
	"testing"
)

func TestExactEntryNames(t *testing.T) {
	entries := []os.DirEntry{fakeEntry{name: "Morning.toml"}}
	if exactEntry(entries, "morning.toml") {
		t.Fatal("matched case variant")
	}
	if !exactEntry(entries, "Morning.toml") {
		t.Fatal("missed exact name")
	}
}

type fakeEntry struct {
	os.DirEntry
	name string
}

func (f fakeEntry) Name() string { return f.name }
