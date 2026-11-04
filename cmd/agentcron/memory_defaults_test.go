package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/agentcron/internal/app"
)

func TestAddMemoryDefaultsToPresenceAndHonorsFlags(t *testing.T) {
	dir := isolate(t)
	if _, err := cli(t, "init"); err != nil {
		t.Fatal(err)
	}
	originalPath := os.Getenv("PATH")
	empty := filepath.Join(dir, "empty-path")
	if err := os.Mkdir(empty, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", empty)
	for _, tc := range []struct {
		name  string
		flags []string
		want  bool
	}{
		{"automatic-off", nil, false}, {"explicit-on", []string{"--memory"}, true}, {"explicit-off", []string{"--no-memory"}, false},
	} {
		args := []string{"add", tc.name, "--schedule", "@daily", "--runner", "claude", "--prompt", "hi"}
		args = append(args, tc.flags...)
		if _, err := cli(t, args...); err != nil {
			t.Fatal(err)
		}
		out, err := cli(t, "show", tc.name, "--json")
		if err != nil {
			t.Fatal(err)
		}
		var detail app.JobDetail
		if err = json.Unmarshal([]byte(out), &detail); err != nil || detail.Job.Memory != tc.want {
			t.Fatalf("%s: %s %v", tc.name, out, err)
		}
	}
	out, err := cli(t, "doctor", "--json")
	if err != nil || !strings.Contains(out, "mdkb is missing") {
		t.Fatalf("%s %v", out, err)
	}
	t.Setenv("PATH", originalPath)
	build(t, filepath.Join(empty, "mdkb"), "../../testdata/fakes/mdkb")
	t.Setenv("PATH", empty+string(os.PathListSeparator)+originalPath)
	if found, err := exec.LookPath("mdkb"); err != nil || found != filepath.Join(empty, "mdkb") {
		t.Fatal("fake mdkb not first")
	}
	if _, err := cli(t, "add", "automatic-on", "--schedule", "@daily", "--runner", "claude", "--prompt", "hi"); err != nil {
		t.Fatal(err)
	}
	out, err = cli(t, "show", "automatic-on", "--json")
	if err != nil || !strings.Contains(out, `"memory":true`) {
		t.Fatalf("%s %v", out, err)
	}
}
