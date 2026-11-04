package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// WorktreeChanges reports files and commits pruning would destroy. A commit is
// protected until it is reachable from a branch outside agentcron's run branches.
func WorktreeChanges(ctx context.Context, path string) (string, error) {
	if _, err := os.Stat(filepath.Join(path, ".git")); os.IsNotExist(err) {
		entries, err := os.ReadDir(path)
		if os.IsNotExist(err) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return strings.Join(names, ", "), nil
	} else if err != nil {
		return "", err
	}
	dirty, err := git(ctx, path, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	refs, err := git(ctx, path, "for-each-ref", "--format=%(refname)", "refs/heads", "refs/remotes")
	if err != nil {
		return "", err
	}
	args := []string{"rev-list", "--oneline", "HEAD", "--not"}
	for _, ref := range strings.Fields(refs) {
		if !strings.HasPrefix(ref, "refs/heads/agentcron/") {
			args = append(args, ref)
		}
	}
	ahead, err := git(ctx, path, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(dirty + "\n" + ahead), nil
}
