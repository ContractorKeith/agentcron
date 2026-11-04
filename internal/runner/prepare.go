// Package runner owns preflight, run directories and child process control.
package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

type prepared struct{ binary, prompt, base string }

func git(ctx context.Context, repo string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}
func preflight(ctx context.Context, j jobs.Job, approved bool, jobsDir string) (prepared, error) {
	p := prepared{prompt: j.Prompt}
	if !approved {
		return p, fmt.Errorf("job is unapproved")
	}
	name := j.Runner
	if name == "command" {
		if len(j.Command) == 0 {
			return p, fmt.Errorf("command is empty")
		}
		name = j.Command[0]
	}
	var err error
	p.binary, err = exec.LookPath(name)
	if err != nil {
		return p, err
	}
	p.binary, err = filepath.Abs(p.binary)
	if err != nil {
		return p, err
	}
	p.prompt, err = jobs.ReadPrompt(jobsDir, j)
	if err != nil {
		return p, err
	}
	if j.Repo != "" {
		inside, e := git(ctx, j.Repo, "rev-parse", "--is-inside-work-tree")
		if e != nil {
			return p, e
		}
		if inside != "true" {
			return p, fmt.Errorf("repo is not a git work tree")
		}
		base := j.Base
		if base == "" {
			base, _ = git(ctx, j.Repo, "symbolic-ref", "refs/remotes/origin/HEAD")
			if base == "" {
				base = "HEAD"
			}
		}
		p.base, err = git(ctx, j.Repo, "rev-parse", "--verify", "--end-of-options", base+"^{commit}")
		if err != nil {
			return p, err
		}
	}
	return p, nil
}
func createWorktree(ctx context.Context, data string, j jobs.Job, r store.Run, p prepared) (string, error) {
	path := filepath.Join(data, "worktrees", j.Name, r.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	if j.Repo == "" {
		return path, os.Mkdir(path, 0700)
	}
	_, err := git(ctx, j.Repo, "worktree", "add", "-b", "agentcron/"+j.Name+"/"+r.ID, path, p.base)
	return path, err
}
func cleanup(ctx context.Context, j jobs.Job, r store.Run, base string) (bool, error) {
	if j.Repo == "" {
		entries, err := os.ReadDir(r.Worktree)
		if err != nil {
			return true, err
		}
		if len(entries) > 0 {
			return true, nil
		}
		return false, os.Remove(r.Worktree)
	}
	dirty, err := git(ctx, r.Worktree, "status", "--porcelain")
	if err != nil {
		return true, err
	}
	ahead, err := git(ctx, r.Worktree, "rev-list", "--count", base+"..HEAD")
	if err != nil {
		return true, err
	}
	if dirty != "" || ahead != "0" {
		return true, nil
	}
	return false, RemoveWorktree(ctx, j.Repo, r)
}
func RemoveWorktree(ctx context.Context, repo string, r store.Run) error {
	if repo == "" {
		return os.RemoveAll(r.Worktree)
	}
	common, err := git(ctx, repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	repo = common
	if _, err := git(ctx, repo, "worktree", "remove", "--force", r.Worktree); err != nil {
		return err
	}
	_, err = git(ctx, repo, "branch", "-D", "agentcron/"+r.Job+"/"+r.ID)
	return err
}
