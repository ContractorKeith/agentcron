package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/runner"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func Logs(ctx context.Context, opts Options, c clock.Clock, id string, stderr, follow bool, out io.Writer) error {
	s, err := openJobs(ctx, opts, c, true)
	if err != nil {
		return err
	}
	defer s.db.Close()
	r, err := s.db.GetRun(ctx, id)
	if err != nil {
		return &Error{Usage, err}
	}
	name := "stdout.log"
	if stderr {
		name = "stderr.log"
	}
	var offset int64
	for {
		f, e := openRegularLog(filepath.Join(s.config.DataDir, "runs"), r.ID, name)
		if e == nil {
			if _, e = f.Seek(offset, io.SeekStart); e == nil {
				var n int64
				n, e = io.Copy(out, f)
				offset += n
			}
			f.Close()
		} else if errors.Is(e, os.ErrNotExist) {
			e = nil
		}
		if e != nil {
			return e
		}
		// Re-read the ledger before the final drain, so output written immediately
		// before terminal state cannot be lost by a follow reader.
		current, e := s.db.GetRun(ctx, id)
		if e != nil {
			return e
		}
		if !follow || store.Terminal(r.State) {
			return nil
		}
		r = current
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.After(50 * time.Millisecond):
		}
	}
}
func Worktrees(ctx context.Context, opts Options, c clock.Clock) ([]Run, error) {
	s, err := openJobs(ctx, opts, c, true)
	if err != nil {
		return nil, err
	}
	defer s.db.Close()
	all, err := s.db.ListRuns(ctx, "", "", -1)
	if err != nil {
		return nil, err
	}
	result := []Run{}
	for _, r := range all {
		if r.Kept || (r.State == "running" && r.Worktree != "") {
			result = append(result, r)
		}
	}
	return result, nil
}
func Prune(ctx context.Context, opts Options, c clock.Clock, dry bool) ([]Run, error) {
	if opts.ReadOnly {
		return nil, &Error{Usage, fmt.Errorf("now preview is read-only")}
	}
	s, err := openJobs(ctx, opts, c, dry)
	if err != nil {
		return nil, err
	}
	defer s.db.Close()
	all, err := s.db.ListRuns(ctx, "", "", -1)
	if err != nil {
		return nil, err
	}
	result := []Run{}
	var warnings []error
	for _, r := range all {
		if !r.Kept || !store.Terminal(r.State) {
			continue
		}
		// Only remove the exact run path managed by this data directory.
		expected := filepath.Join(s.config.DataDir, "worktrees", r.Job, r.ID)
		if r.Worktree != expected {
			warnings = append(warnings, fmt.Errorf("unexpected worktree path %q", r.Worktree))
			continue
		}
		result = append(result, r)
		if dry {
			continue
		}
		repo := ""
		// A worktree's own .git metadata identifies its repository even after rm job.
		if _, e := os.Stat(filepath.Join(r.Worktree, ".git")); e == nil {
			repo = r.Worktree
		}
		if err = runner.RemoveWorktree(ctx, repo, r); err != nil {
			return nil, err
		}
		if err = s.db.ClearKept(ctx, r.ID); err != nil {
			return nil, err
		}
	}
	return result, errors.Join(warnings...)
}
