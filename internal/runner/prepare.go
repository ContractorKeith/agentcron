// Package runner owns preflight, run directories and child process control.
package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

type prepared struct {
	binary, prompt, base, configDir string
	memory                          runMemory
}

func git(ctx context.Context, repo string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}
func preflight(ctx context.Context, j jobs.Job, approved bool, jobsDir string) (prepared, error) {
	p := prepared{prompt: j.Prompt}
	if jobsDir != "" {
		p.configDir = filepath.Dir(jobsDir)
	}
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
	if err := configOutside(path, p.configDir); err != nil {
		return "", err
	}
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

// rollbackWorktree handles partial add failures, including post-checkout hooks.
func rollbackWorktree(ctx context.Context, j jobs.Job, r store.Run) (bool, error) {
	if r.Worktree == "" {
		return false, nil
	}
	if j.Repo == "" {
		err := os.Remove(r.Worktree)
		if os.IsNotExist(err) {
			err = nil
		}
		return err != nil, err
	}
	_, removeErr := git(ctx, j.Repo, "worktree", "remove", "--force", r.Worktree)
	branch := "agentcron/" + r.Job + "/" + r.ID
	_, branchErr := git(ctx, j.Repo, "branch", "-D", branch)
	trees, listErr := git(ctx, j.Repo, "worktree", "list", "--porcelain")
	_, pathErr := os.Stat(r.Worktree)
	_, refErr := git(ctx, j.Repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if os.IsNotExist(pathErr) && refErr != nil && listErr == nil && !strings.Contains(trees, "worktree "+r.Worktree+"\n") {
		return false, nil
	}
	return true, errors.Join(removeErr, branchErr, listErr)
}

// Resolve existing ancestors before checking containment of a future worktree.
func configOutside(worktree, configDir string) error {
	if configDir == "" {
		return nil
	}
	resolve := func(path string) (string, error) {
		path, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		var suffix []string
		for {
			real, e := filepath.EvalSymlinks(path)
			if e == nil {
				for i := len(suffix) - 1; i >= 0; i-- {
					real = filepath.Join(real, suffix[i])
				}
				return real, nil
			}
			if !os.IsNotExist(e) {
				return "", e
			}
			parent := filepath.Dir(path)
			if parent == path {
				return "", e
			}
			suffix = append(suffix, filepath.Base(path))
			path = parent
		}
	}
	root, err := resolve(worktree)
	if err != nil {
		return err
	}
	target, err := resolve(configDir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("run worktree would contain the config directory; move config outside worktrees")
	}
	return nil
}
