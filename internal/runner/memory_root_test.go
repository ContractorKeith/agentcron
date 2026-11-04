package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryExistingNoteRootChangeAndSlug(t *testing.T) {
	x, j := setup(t)
	j.Memory = true
	for _, name := range []string{"fixture", "trailing-", "double--dash", strings.Repeat("a", 41)} {
		j.Name = name
		root := filepath.Join(x.DataDir, "kb-"+name)
		t.Setenv("MDKB_ROOT", root)
		if err := os.MkdirAll(filepath.Join(root, ".mdkb"), 0700); err != nil {
			t.Fatal(err)
		}
		r := executeRun(t, x, j, true)
		if r.State != "succeeded" {
			t.Fatal(r)
		}
		if name != "fixture" {
			if !strings.Contains(r.Error, "cannot create the exact note") {
				t.Fatal(r)
			}
			continue
		}
		// A different configured root must not reuse the first KB's bootstrap fact.
		second := filepath.Join(root, "second")
		t.Setenv("MDKB_ROOT", second)
		if err := os.MkdirAll(filepath.Join(second, "agentcron"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(second, ".mdkb"), 0700); err != nil {
			t.Fatal(err)
		}
		note := filepath.Join(second, "agentcron", name+".md")
		if err := os.WriteFile(note, []byte("existing note\n"), 0600); err != nil {
			t.Fatal(err)
		}
		r = executeRun(t, x, j, true)
		if r.State != "succeeded" || r.Error != "" {
			t.Fatal(r)
		}
		data, err := os.ReadFile(note)
		if err != nil || string(data) != "existing note\n" {
			t.Fatalf("%q %v", data, err)
		}
	}
}
