package mcpserver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRunUnavailableAndUnknownRunFilter(t *testing.T) {
	cs, opts, c := fixture(t, true)
	call(t, cs, "create_job", map[string]any{"name": "paused", "schedule": "@daily", "runner": "command", "command": []string{"cat"}, "prompt": "hi"}, false)
	if err := app.ApproveJob(t.Context(), opts, c, "paused"); err != nil {
		t.Fatal(err)
	}
	if err := app.PauseJob(t.Context(), opts, c, "paused", true); err != nil {
		t.Fatal(err)
	}
	call(t, cs, "run_job", map[string]any{"name": "paused"}, true)
	call(t, cs, "list_runs", map[string]any{"job": "missing"}, true)
	runs, err := app.Runs(t.Context(), opts, c, "paused", "", 50)
	if err != nil || len(runs) != 0 {
		t.Fatal(runs, err)
	}
	if err := os.Remove(filepath.Join(opts.ConfigDir, "jobs", "paused.toml")); err != nil {
		t.Fatal(err)
	}
	call(t, cs, "run_job", map[string]any{"name": "paused"}, true)
	if err := os.WriteFile(filepath.Join(opts.ConfigDir, "jobs", "paused.toml"), []byte("invalid = ["), 0600); err != nil {
		t.Fatal(err)
	}
	call(t, cs, "run_job", map[string]any{"name": "paused"}, true)
}

func TestPauseResumeDescriptions(t *testing.T) {
	cs, _, _ := fixture(t, true)
	list, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	descriptions := map[string]string{}
	for _, tool := range list.Tools {
		descriptions[tool.Name] = tool.Description
	}
	for _, name := range []string{"pause_job", "resume_job"} {
		verb := strings.TrimSuffix(name, "_job")
		if !strings.HasPrefix(strings.ToLower(descriptions[name]), verb+" ") {
			t.Fatalf("%s: %s", name, descriptions[name])
		}
	}
	if descriptions["pause_job"] == descriptions["resume_job"] {
		t.Fatal("identical descriptions")
	}
}

func TestLogSymlinkIsToolError(t *testing.T) {
	cs, opts, c := fixture(t, true)
	call(t, cs, "create_job", map[string]any{"name": "output", "schedule": "@daily", "runner": "command", "command": []string{"cat"}, "prompt": "hi"}, false)
	if err := app.ApproveJob(t.Context(), opts, c, "output"); err != nil {
		t.Fatal(err)
	}
	r, err := app.EnqueueMCPRun(t.Context(), opts, c, "output")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(opts.DataDir, "runs", r.ID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(dir, "stdout.log")); err != nil {
		t.Fatal(err)
	}
	result := call(t, cs, "get_run_output", map[string]any{"id": r.ID, "stream": "stdout"}, true)
	if !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "log is not a regular file") {
		t.Fatal(result)
	}
	call(t, cs, "list_jobs", map[string]any{}, false)
}

func TestUnknownJobRunFilterIsToolError(t *testing.T) {
	cs, _, _ := fixture(t, false)
	call(t, cs, "list_runs", map[string]any{"job": "unknown"}, true)
}
