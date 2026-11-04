package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/store"
)

func TestOutcomes(t *testing.T) {
	x, j := setup(t)
	for _, tc := range []struct{ mode, expect, state string }{{"ok", "DONE", "succeeded"}, {"fail", "", "failed"}, {"ok", "missing", "attention"}, {"ignore-term", "", "timed_out"}, {"spawn-child", "", "timed_out"}, {"noisy", "", "succeeded"}} {
		t.Run(tc.mode+tc.state, func(t *testing.T) {
			t.Setenv("FAKE_MODE", tc.mode)
			if tc.mode == "spawn-child" {
				pidFile := filepath.Join(x.DataDir, "outcome-child")
				t.Setenv("FAKE_CHILD_PID", pidFile)
				defer func() {
					b, _ := os.ReadFile(pidFile)
					pid, _ := strconv.Atoi(string(b))
					if pid > 0 {
						_ = syscall.Kill(pid, syscall.SIGKILL)
						reapDescendant(pid)
					}
				}()
			}
			t.Setenv("FAKE_NOISY_STREAM", "both")
			j.Expect = tc.expect
			j.Timeout = "5s"
			if tc.state == "timed_out" {
				j.Timeout = "150ms"
			}
			r := executeRun(t, x, j, true)
			if r.State != tc.state {
				t.Fatalf("%+v", r)
			}
			if r.Kept {
				t.Fatal("empty scratch kept")
			}
			if tc.mode == "noisy" {
				for _, stream := range []string{"stdout", "stderr"} {
					info, e := os.Stat(filepath.Join(x.DataDir, "runs", r.ID, stream+".log"))
					if e != nil || info.Size() != 10*1024*1024 {
						t.Fatalf("%v %v", info, e)
					}
				}
			}
		})
	}
}
func TestAdaptersAndEnvironment(t *testing.T) {
	x, j := setup(t)
	for _, name := range []string{"claude", "codex", "command"} {
		t.Run(name, func(t *testing.T) {
			j.Runner = name
			if name == "command" {
				j.Command = []string{"claude", "first"}
				t.Setenv("FAKE_STDIN", "1")
			}
			record := filepath.Join(x.DataDir, "record-"+name)
			t.Setenv("FAKE_RECORD", record)
			j.Args = []string{"--permission-mode", "acceptEdits", "two words"}
			r := executeRun(t, x, j, true)
			if r.State != "succeeded" {
				t.Fatal(r)
			}
			b, e := os.ReadFile(record)
			if e != nil {
				t.Fatal(e)
			}
			var got struct {
				Argv       []string
				Cwd, Stdin string
				Env        map[string]string
			}
			if e = json.Unmarshal(b, &got); e != nil {
				t.Fatal(e)
			}
			last, e := x.DB.ListRuns(context.Background(), j.Name, "", 2)
			if e != nil {
				t.Fatal(e)
			}
			var prev *store.Run
			if len(last) > 1 {
				prev = &last[1]
			}
			prompt := Preamble(r, prev, x.DataDir) + j.Prompt
			var want []string
			switch name {
			case "claude":
				want = append([]string{"-p", prompt, "--output-format", "text"}, j.Args...)
			case "codex":
				want = append(append([]string{"exec", "-C", r.Worktree}, j.Args...), prompt)
			case "command":
				want = append([]string{"first"}, j.Args...)
			}
			if !reflect.DeepEqual(got.Argv[1:], want) {
				t.Fatalf("argv: %q", got.Argv)
			}
			if name == "command" && got.Stdin != prompt {
				t.Fatalf("stdin %q", got.Stdin)
			}
			if name != "command" && got.Stdin != "" {
				t.Fatal("agent stdin not empty")
			}
			root, e := filepath.EvalSymlinks(x.DataDir)
			if e != nil {
				t.Fatal(e)
			}
			if !strings.HasPrefix(got.Cwd, root+string(filepath.Separator)) {
				t.Fatal(got.Cwd)
			}
			for key, want := range map[string]string{"AGENTCRON_JOB": j.Name, "AGENTCRON_RUN_ID": r.ID, "AGENTCRON_WORKDIR": r.Worktree, "AGENTCRON_SCHEDULED_FOR": r.ScheduledFor.Format(time.RFC3339Nano)} {
				if got.Env[key] != want {
					t.Fatalf("%s=%s", key, got.Env[key])
				}
			}
			if _, ok := got.Env["AGENTCRON_MEMORY_NOTE"]; ok {
				t.Fatal("memory injected before M4")
			}
		})
	}
}
func TestWorktrees(t *testing.T) {
	x, j := setup(t)
	j.Repo = repo(t, x)
	r := executeRun(t, x, j, true)
	if r.State != "succeeded" || r.Kept {
		t.Fatal(r)
	}
	if _, e := os.Stat(r.Worktree); !os.IsNotExist(e) {
		t.Fatal("clean directory remains")
	}
	branches, e := git(context.Background(), j.Repo, "branch", "--list", "agentcron/*")
	if e != nil || branches != "" {
		t.Fatalf("%s %v", branches, e)
	}
	t.Setenv("FAKE_MODE", "dirty")
	r = executeRun(t, x, j, true)
	if !r.Kept {
		t.Fatal(r)
	}
	if e = RemoveWorktree(context.Background(), r.Worktree, r); e != nil {
		t.Fatal(e)
	}
}

func TestAheadAndNonemptyScratchKept(t *testing.T) {
	x, j := setup(t)
	t.Setenv("FAKE_MODE", "dirty")
	r := executeRun(t, x, j, true)
	if !r.Kept {
		t.Fatal("nonempty scratch removed")
	}
	if e := RemoveWorktree(context.Background(), "", r); e != nil {
		t.Fatal(e)
	}
	j.Repo = repo(t, x)
	j.Runner = "command"
	j.Command = []string{"git", "commit", "--allow-empty", "-m", "run commit"}
	r = executeRun(t, x, j, true)
	if r.State != "succeeded" || !r.Kept {
		t.Fatal(r)
	}
	if e := RemoveWorktree(context.Background(), j.Repo, r); e != nil {
		t.Fatal(e)
	}
}
