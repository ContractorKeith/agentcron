package runner

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

type Runner struct {
	DB                          *store.Store
	Clock                       clock.Clock
	DataDir, JobsDir, ConfigDir string
	Grace                       time.Duration
}

func (x Runner) Run(ctx context.Context, j jobs.Job, r store.Run, approved bool) (store.Run, error) {
	if x.Grace == 0 {
		x.Grace = 10 * time.Second
	}
	// Finalization must survive caller cancellation.
	persist := context.WithoutCancel(ctx)
	running := false
	finish := func(state string, cause error) (store.Run, error) {
		r.State = state
		t := x.Clock.Now().UTC()
		r.EndedAt = &t
		if cause != nil {
			r.Error = cause.Error()
		}
		var err error
		if !running && (state == "failed" || state == "timed_out") {
			err = x.DB.FinishClaimedRun(persist, r)
		} else {
			err = x.DB.FinishRun(persist, r)
		}
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
	activeRuns, err := x.DB.ListRuns(ctx, j.Name, "running", -1)
	if err != nil {
		return r, err
	}
	for _, active := range activeRuns {
		if identityLive(active) {
			return finish("skipped", nil)
		}
	}
	last, err := x.DB.PreviousRun(ctx, r)
	if err != nil {
		return r, err
	}
	if err = x.DB.ClaimRun(ctx, r.ID); err != nil {
		return r, err
	}
	setupStarted := x.Clock.Now()
	p, updated, err := x.setup(ctx, j, r, approved)
	r = updated
	if err != nil {
		state := "blocked_config"
		if errors.Is(err, errSetupTimeout) {
			state = "timed_out"
		}
		return finish(state, err)
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
	inputPath := os.DevNull
	if j.Runner == "command" {
		inputPath = filepath.Join(dir, "prompt.txt")
		if err = os.WriteFile(inputPath, []byte(prompt), 0600); err != nil {
			return abort(err)
		}
	}
	input, err := os.Open(inputPath)
	if err != nil {
		return abort(err)
	}
	defer input.Close()
	cmd.Stdin = input
	lastOutput := ""
	if last != nil {
		lastOutput = filepath.Join(x.DataDir, "runs", last.ID, "stdout.log")
	}
	cmd.Env = x.runEnv(j, r, lastOutput)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	started := x.Clock.Now().UTC()
	if err = cmd.Start(); err != nil {
		return abort(err)
	}
	pid := cmd.Process.Pid
	if actual, e := ProcessStart(pid); e == nil {
		r.ProcStart = &actual
	}
	owner := os.Getpid()
	r.OwnerPID = &owner
	r.PID = &pid
	r.StartedAt = &started
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	if err = x.DB.StartOwnedRun(persist, r); err != nil {
		_ = terminate(pid, done, x.Clock, x.Grace)
		r.Kept, _ = cleanup(persist, j, r, p.base)
		return finish("failed", err)
	}
	running = true
	timeout, _ := time.ParseDuration(j.Timeout)
	state := "succeeded"
	var waitErr error
	timer := x.Clock.After(timeout - x.Clock.Now().Sub(setupStarted))
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
