package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMemoryRecreatesDeletedNoteAndRetargetedKB(t *testing.T) {
	x, j := setup(t)
	j.Memory = true
	first, second := filepath.Join(x.DataDir, "first"), filepath.Join(x.DataDir, "second")
	for _, root := range []string{first, second} {
		if err := os.MkdirAll(filepath.Join(root, ".mdkb"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(x.DataDir, "kb-link")
	if err := os.Symlink(first, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MDKB_ROOT", link)
	for i := 0; i < 3; i++ {
		r := executeRun(t, x, j, true)
		if r.State != "succeeded" || r.Error != "" {
			t.Fatal(r)
		}
		if _, err := os.Stat(filepath.Join(link, "agentcron", j.Name+".md")); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if err := os.Remove(filepath.Join(first, "agentcron", j.Name+".md")); err != nil {
				t.Fatal(err)
			}
		}
		if i == 1 {
			if err := os.Remove(link); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(second, link); err != nil {
				t.Fatal(err)
			}
		}
	}
}
