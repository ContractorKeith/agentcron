package runner

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMemoryRunnerPermissionsAndOwnedCodexConfig(t *testing.T) {
	x, j := setup(t)
	j.Memory = true
	if err := os.MkdirAll(filepath.Join(os.Getenv("MDKB_ROOT"), ".mdkb"), 0700); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(x.DataDir, "record")
	t.Setenv("FAKE_RECORD", record)
	r := executeRun(t, x, j, true)
	if r.State != "succeeded" || r.Error != "" {
		t.Fatal(r)
	}
	entries := records(t, record)
	argv := entries[len(entries)-1].Argv
	want := []string{"--output-format", "text", "--mcp-config", filepath.Join(x.DataDir, "runs", r.ID, "mcp.json"), "--strict-mcp-config", "--allowedTools", "mcp__mdkb__*"}
	if len(argv) < 3 || argv[1] != "-p" || !reflect.DeepEqual(argv[3:], want) {
		t.Fatalf("%q", argv)
	}
	j.Runner = "codex"
	j.Args = []string{"-c", `mcp_servers.mdkb.command="custom"`}
	r = executeRun(t, x, j, true)
	if r.State != "succeeded" || r.Error != "" {
		t.Fatal(r)
	}
	entries = records(t, record)
	argv = entries[len(entries)-1].Argv
	if !reflect.DeepEqual(argv[1:len(argv)-1], append([]string{"exec", "-C", r.Worktree}, j.Args...)) || !strings.Contains(argv[len(argv)-1], "job's own mcp_servers.mdkb") {
		t.Fatalf("%q", argv)
	}
}

func TestInjectedAllowancePreservesJobToolPermissions(t *testing.T) {
	x, j := setup(t)
	j.Memory = true
	j.Args = []string{"--allowedTools", "Bash(git *)", "Edit"}
	if err := os.MkdirAll(filepath.Join(os.Getenv("MDKB_ROOT"), ".mdkb"), 0700); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(x.DataDir, "record")
	t.Setenv("FAKE_RECORD", record)
	r := executeRun(t, x, j, true)
	if r.State != "succeeded" || r.Error != "" {
		t.Fatal(r)
	}
	entries := records(t, record)
	argv := entries[len(entries)-1].Argv
	count := 0
	for i, arg := range argv {
		if arg == "--allowedTools" {
			count++
			if i+2 >= len(argv) || argv[i+1] != "Bash(git *),mcp__mdkb__*" || argv[i+2] != "Edit" {
				t.Fatalf("%q", argv)
			}
		}
	}
	if count != 1 {
		t.Fatalf("conflicting allowance flags: %q", argv)
	}
}
