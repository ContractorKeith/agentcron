package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/ContractorKeith/agentcron/internal/memory"
)

type memoryRecord struct {
	Argv []string
	Env  map[string]string
}

func records(t *testing.T, path string) []memoryRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result []memoryRecord
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var r memoryRecord
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatal(err)
		}
		result = append(result, r)
	}
	return result
}

func TestMemoryRuns(t *testing.T) {
	x, j := setup(t)
	j.Memory = true
	root := filepath.Join(x.DataDir, "kb")
	if err := os.MkdirAll(filepath.Join(root, ".mdkb"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MDKB_ROOT", root)
	record := filepath.Join(x.DataDir, "memory-record")
	t.Setenv("FAKE_RECORD", record)
	for _, runner := range []string{"claude", "codex", "command", "claude"} {
		j.Runner = runner
		j.Command = nil
		if runner == "command" {
			j.Command = []string{"claude"}
		}
		r := executeRun(t, x, j, true)
		if r.State != "succeeded" || r.Error != "" {
			t.Fatalf("%+v", r)
		}
		all := records(t, record)
		got := all[len(all)-1]
		if got.Env["AGENTCRON_MEMORY_NOTE"] != "agentcron/fixture" || got.Env["MDKB_ROOT"] != root {
			t.Fatal(got)
		}
		switch runner {
		case "claude":
			data, err := os.ReadFile(filepath.Join(x.DataDir, "runs", r.ID, "mcp.json"))
			if err != nil {
				t.Fatal(err)
			}
			var config struct {
				MCPServers map[string]struct {
					Command string
					Args    []string
				}
			}
			if err = json.Unmarshal(data, &config); err != nil {
				t.Fatal(err)
			}
			server := config.MCPServers["mdkb"]
			if server.Command != filepath.Join(x.DataDir, "bin", "mdkb") || strings.Join(server.Args, " ") != "--root "+root+" mcp --allow-write" {
				t.Fatal(server)
			}
		case "codex":
			overrides := map[string]any{}
			for i, arg := range got.Argv {
				if arg == "-c" {
					if _, err := toml.Decode(got.Argv[i+1], &overrides); err != nil {
						t.Fatal(err)
					}
				}
			}
			if !strings.Contains(strings.Join(got.Argv, " "), "mcp_servers.mdkb.command=") {
				t.Fatal(got)
			}
		}
	}
	newCount, catCount := 0, 0
	for _, r := range records(t, record) {
		if filepath.Base(r.Argv[0]) == "mdkb" {
			for _, a := range r.Argv[1:] {
				if a == "new" {
					newCount++
				}
				if a == "cat" {
					catCount++
				}
			}
		}
	}
	if newCount != 1 || catCount != 1 {
		t.Fatalf("new=%d cat=%d", newCount, catCount)
	}
	client, err := memory.Resolve(filepath.Join(x.DataDir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	health := client.Status(context.Background())
	if !health.Present || !health.Initialised || health.Version != "mdkb fake-v1" {
		t.Fatal(health)
	}
}

func TestMemoryFailureAndOwnConfig(t *testing.T) {
	x, j := setup(t)
	j.Memory = true
	root := filepath.Join(x.DataDir, "kb")
	t.Setenv("MDKB_ROOT", root)
	r := executeRun(t, x, j, true)
	if r.State != "succeeded" || !strings.Contains(r.Error, "memory unavailable:") {
		t.Fatalf("%+v", r)
	}
	if err := os.MkdirAll(filepath.Join(root, ".mdkb"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--mcp-config", "custom.json"}, {"--mcp-config=custom.json"}} {
		j.Args = args
		record := filepath.Join(x.DataDir, r.ID+"record")
		t.Setenv("FAKE_RECORD", record)
		r = executeRun(t, x, j, true)
		all := records(t, record)
		got := all[len(all)-1]
		if r.State != "succeeded" || r.Error != "" {
			t.Fatal(r)
		}
		if strings.Contains(strings.Join(got.Argv, " "), "--strict-mcp-config") || !strings.Contains(got.Argv[2], "job's own --mcp-config") {
			t.Fatal(got)
		}
		if _, err := os.Stat(filepath.Join(x.DataDir, "runs", r.ID, "mcp.json")); !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	// An unavailable configured binary overrides even the available fake on PATH.
	if err := os.WriteFile(filepath.Join(filepath.Dir(x.JobsDir), "config.toml"), []byte("mdkb_path = '/missing/mdkb'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r = executeRun(t, x, j, true)
	if r.State != "succeeded" || !strings.Contains(r.Error, "memory unavailable:") {
		t.Fatal(r)
	}
	all := records(t, os.Getenv("FAKE_RECORD"))
	got := all[len(all)-1]
	if got.Env["AGENTCRON_MEMORY_NOTE"] != "" || strings.Contains(got.Argv[2], "Read it with the mdkb MCP") {
		t.Fatal(got)
	}
	t.Setenv("FAKE_MODE", "fail")
	r = executeRun(t, x, j, true)
	if r.State != "failed" || !strings.Contains(r.Error, "memory unavailable:") || !strings.Contains(r.Error, "exit status 1") {
		t.Fatal(r)
	}
}
