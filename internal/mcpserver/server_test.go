package mcpserver_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func fixture(t *testing.T, writes bool) (*mcp.ClientSession, app.Options, *clock.Fake) {
	t.Helper()
	root := t.TempDir()
	opts := app.Options{ConfigDir: filepath.Join(root, "config"), DataDir: filepath.Join(root, "data")}
	c := clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
	if err := app.Init(t.Context(), opts); err != nil {
		t.Fatal(err)
	}
	s, err := mcpserver.New(t.Context(), opts, c, writes, "test")
	if err != nil {
		t.Fatal(err)
	}
	a, b := mcp.NewInMemoryTransports()
	ss, err := s.Connect(t.Context(), a, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(t.Context(), b, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs, opts, c
}
func call(t *testing.T, cs *mcp.ClientSession, name string, args any, wantError bool) *mcp.CallToolResult {
	t.Helper()
	r, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if r.IsError != wantError {
		t.Fatalf("%s: %+v", name, r)
	}
	return r
}
func normalized(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err = json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestToolsAndApproval(t *testing.T) {
	for _, writes := range []bool{false, true} {
		cs, opts, c := fixture(t, writes)
		list, err := cs.ListTools(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		want := 5
		if writes {
			want = 9
		}
		if len(list.Tools) != want {
			t.Fatal(list)
		}
		if !writes {
			r, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "create_job", Arguments: map[string]any{}})
			if err == nil && !r.IsError {
				t.Fatal("write registered")
			}
			continue
		}
		args := map[string]any{"name": "agent", "schedule": "every 1m", "runner": "command", "command": []string{"cat"}, "prompt": "hi"}
		call(t, cs, "create_job", args, false)
		detail, err := app.ShowJob(t.Context(), opts, c, "agent")
		if err != nil || detail.Status != "unapproved" || !detail.State.Paused || detail.Job.CreatedBy != "mcp" {
			t.Fatal(detail, err)
		}
		for _, name := range []string{"pause_job", "resume_job"} {
			call(t, cs, name, map[string]any{"name": "agent"}, false)
		}
		call(t, cs, "run_job", map[string]any{"name": "agent"}, true)
		reads := []struct {
			name string
			args any
			want any
		}{
			{"get_job", map[string]any{"name": "agent"}, nil},
			{"list_jobs", map[string]any{}, nil},
			{"list_runs", map[string]any{}, nil},
			{"next_fires", map[string]any{}, nil},
		}
		reads[0].want, _ = app.ShowJob(t.Context(), opts, c, "agent")
		reads[1].want, _ = app.ListJobs(t.Context(), opts, c)
		reads[2].want, _ = app.Runs(t.Context(), opts, c, "", "", 50)
		reads[3].want, _ = app.NextFires(t.Context(), opts, c, "", "", 5)
		for _, tt := range reads {
			got := call(t, cs, tt.name, tt.args, false).StructuredContent
			if !reflect.DeepEqual(normalized(t, got), normalized(t, tt.want)) {
				t.Fatalf("%s: %+v != %+v", tt.name, got, tt.want)
			}
		}
		for range 2 {
			if err := app.ApproveJob(t.Context(), opts, c, "agent"); err != nil {
				t.Fatal(err)
			}
		}
		result := call(t, cs, "run_job", map[string]any{"name": "agent"}, false)
		run := normalized(t, result.StructuredContent).(map[string]any)
		if run["trigger"] != "mcp" || run["state"] != "queued" {
			t.Fatal(run)
		}
		id := run["id"].(string)
		dir := filepath.Join(opts.DataDir, "runs", id)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		for _, stream := range []string{"stdout", "stderr"} {
			if err := os.WriteFile(filepath.Join(dir, stream+".log"), []byte("abcdef"), 0600); err != nil {
				t.Fatal(err)
			}
			r := call(t, cs, "get_run_output", map[string]any{"id": id, "stream": stream, "tail_bytes": 3}, false)
			out := normalized(t, r.StructuredContent).(map[string]any)
			if out["output"] != "def" || out["truncated"] != true {
				t.Fatal(out)
			}
			r = call(t, cs, "get_run_output", map[string]any{"id": id, "stream": stream}, false)
			out = normalized(t, r.StructuredContent).(map[string]any)
			if out["output"] != "abcdef" || out["truncated"] != false {
				t.Fatal(out)
			}
		}
		for _, tt := range []struct {
			name string
			args any
		}{
			{"list_runs", map[string]any{"limit": 201}}, {"list_runs", map[string]any{"state": "bad"}},
			{"next_fires", map[string]any{"count": 51}}, {"get_job", map[string]any{"name": "missing"}},
			{"get_run_output", map[string]any{"id": id, "stream": "bad"}},
			{"get_run_output", map[string]any{"id": id, "stream": "stdout", "tail_bytes": 262145}},
			{"create_job", args},
		} {
			call(t, cs, tt.name, tt.args, true)
		}
	}
}
func TestRecursionGuard(t *testing.T) {
	for _, allow := range []bool{false, true} {
		cs, opts, _ := fixture(t, true)
		t.Setenv("AGENTCRON_RUN_ID", "scheduled")
		if allow {
			if err := os.WriteFile(filepath.Join(opts.ConfigDir, "config.toml"), []byte("allow_agent_scheduling = true\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		r := call(t, cs, "create_job", map[string]any{"name": "nested", "schedule": "@daily", "runner": "command", "command": []string{"cat"}, "prompt": "hi"}, !allow)
		if !allow && !strings.Contains(r.Content[0].(*mcp.TextContent).Text, "allow_agent_scheduling") {
			t.Fatal(r)
		}
		for _, name := range []string{"pause_job", "resume_job", "run_job"} {
			r = call(t, cs, name, map[string]any{"name": "missing"}, true)
			text := r.Content[0].(*mcp.TextContent).Text
			if strings.Contains(text, "allow_agent_scheduling") == allow {
				t.Fatal(text)
			}
		}
	}
}
