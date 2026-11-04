package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ContractorKeith/agentcron/internal/config"
	"github.com/ContractorKeith/agentcron/internal/jobs"
)

func trustWarnings(c config.Config) ([]string, error) {
	if !isDir(c.JobsDir()) {
		return nil, nil
	}
	entries, err := jobs.List(c.JobsDir())
	if err != nil {
		return nil, err
	}
	var warnings []string
	target, err := filepath.EvalSymlinks(c.JobsDir())
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Job == nil || e.Job.Repo == "" {
			continue
		}
		repo, err := filepath.EvalSymlinks(e.Job.Repo)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(repo, target)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			warnings = append(warnings, fmt.Sprintf("job %s: jobs directory is inside repo %s; runner permissions must prevent writing the config directory", e.Name, repo))
		}
	}
	return warnings, nil
}
