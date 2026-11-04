package runner

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func (x Runner) runEnv(j jobs.Job, r store.Run, last string) []string {
	env := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "AGENTCRON_") {
			env = append(env, entry)
		}
	}
	configDir := x.ConfigDir
	if configDir == "" {
		configDir = filepath.Dir(x.JobsDir)
	}
	return append(env, "AGENTCRON_CONFIG_DIR="+configDir, "AGENTCRON_DATA_DIR="+x.DataDir, "AGENTCRON_JOB="+j.Name, "AGENTCRON_RUN_ID="+r.ID, "AGENTCRON_SCHEDULED_FOR="+r.ScheduledFor.Format(time.RFC3339Nano), "AGENTCRON_WORKDIR="+r.Worktree, "AGENTCRON_LAST_OUTPUT="+last)
}
