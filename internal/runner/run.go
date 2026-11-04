package runner

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

type Runner struct {
	DB               *store.Store
	Clock            clock.Clock
	DataDir, JobsDir string
	Grace            time.Duration
}

func (x Runner) Run(ctx context.Context, j jobs.Job, r store.Run, approved bool) (store.Run, error) {
	if x.Grace == 0 {
		x.Grace = 10 * time.Second
	}
	// Finalization must survive caller cancellation.
	persist := context.WithoutCancel(ctx)
	finish := func(state string, cause error) (store.Run, error) {
		r.State = state
		t := x.Clock.Now().UTC()
		r.EndedAt = &t
		if cause != nil {
			r.Error = cause.Error()
		}
		err := x.DB.FinishRun(persist, r)
		return r, err
	}
	f, acquired, err := lock(x.DataDir, j.Name)
	if err != nil {
		return r, err
	}
	if !acquired {
		return finish("skipped", nil)
	}
	defer f.Close()
	last, err := x.DB.PreviousRun(ctx, r)
	if err != nil {
		return r, err
	}
	if err = x.DB.ClaimRun(ctx, r.ID); err != nil {
		return r, err
	}
	p, err := preflight(ctx, j, approved, x.JobsDir)
	if err != nil {
		return finish("blocked_config", err)
	}
	r.Worktree, err = createWorktree(ctx, x.DataDir, j, r, p)
	if err != nil {
		return finish("blocked_config", err)
	}
	abort := func(cause error) (store.Run, error) {
		r.Kept, err = cleanup(persist, j, r, p.base)
		return finish("blocked_config", errors.Join(cause, err))
	}
	dir := filepath.Join(x.DataDir, "runs", r.ID)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return abort(err)
	}
	stdout, err := os.OpenFile(filepath.Join(dir, "stdout.log"), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return abort(err)
	}
	defer stdout.Close()
	stderr, err := os.OpenFile(filepath.Join(dir, "stderr.log"), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return abort(err)
	}
	defer stderr.Close()
	prompt := Preamble(r, last, x.DataDir) + p.prompt
	cmd := exec.Command(p.binary, Argv(j, r.Worktree, prompt)...)
	cmd.Dir = r.Worktree
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if j.Runner == "command" {
		cmd.Stdin = strings.NewReader(prompt)
	}
	lastOutput := ""
	if last != nil {
		lastOutput = filepath.Join(x.DataDir, "runs", last.ID, "stdout.log")
	}
	cmd.Env = runEnv(j, r, lastOutput)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	started := x.Clock.Now().UTC().Truncate(time.Second)
	if err = cmd.Start(); err != nil {
		return abort(err)
	}
	pid := cmd.Process.Pid
	if actual, e := ProcessStart(pid); e == nil {
		started = actual
	}
	r.PID = &pid
	r.StartedAt = &started
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	if err = x.DB.StartRun(persist, r.ID, pid, started, r.Worktree); err != nil {
		_ = terminate(pid, done, x.Clock, x.Grace)
		return r, err
	}
	timeout, _ := time.ParseDuration(j.Timeout)
	state := "succeeded"
	var waitErr error
	timer := x.Clock.After(timeout)
	active := true
	for active {
		select {
		case waitErr = <-done:
			teardownGroup(pid, x.Clock, x.Grace)
			active = false
		case <-ctx.Done():
			state = "cancelled"
			waitErr = terminate(pid, done, x.Clock, x.Grace)
			active = false
		case <-timer:
			state = "timed_out"
			waitErr = terminate(pid, done, x.Clock, x.Grace)
			active = false
		case <-x.Clock.After(50 * time.Millisecond):
			if _, e := os.Stat(filepath.Join(dir, "cancel")); e == nil {
				state = "cancelled"
				waitErr = terminate(pid, done, x.Clock, x.Grace)
				active = false
			}
		}
	}
	if _, e := os.Stat(filepath.Join(dir, "cancel")); e == nil && state == "succeeded" {
		state = "cancelled"
	}
	code := cmd.ProcessState.ExitCode()
	r.ExitCode = &code
	if state == "succeeded" && waitErr != nil {
		state = "failed"
	}
	if state == "succeeded" && j.Expect != "" {
		file, e := os.Open(filepath.Join(dir, "stdout.log"))
		if e != nil {
			waitErr = e
			state = "failed"
		} else {
			// MatchReader streams the file; no unbounded output buffer.
			re := regexp.MustCompile(j.Expect)
			matched := re.MatchReader(bufio.NewReader(file))
			file.Close()
			if !matched {
				state = "attention"
			}
		}
	}
	kept, cleanErr := cleanup(persist, j, r, p.base)
	r.Kept = kept
	return finish(state, errors.Join(waitErr, cleanErr))
}
func runEnv(j jobs.Job, r store.Run, last string) []string {
	env := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "AGENTCRON_") {
			env = append(env, entry)
		}
	}
	return append(env, "AGENTCRON_JOB="+j.Name, "AGENTCRON_RUN_ID="+r.ID, "AGENTCRON_SCHEDULED_FOR="+r.ScheduledFor.Format(time.RFC3339Nano), "AGENTCRON_WORKDIR="+r.Worktree, "AGENTCRON_LAST_OUTPUT="+last)
}
