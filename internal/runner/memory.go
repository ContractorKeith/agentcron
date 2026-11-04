package runner

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/memory"
	"github.com/ContractorKeith/agentcron/internal/store"
)

type runMemory struct {
	preamble   string
	flags, env []string
}

func (x Runner) prepareMemory(ctx context.Context, j jobs.Job, dir string) (runMemory, error) {
	var result runMemory
	if !j.Memory {
		return result, nil
	}
	configDir := x.ConfigDir
	if configDir == "" {
		configDir = filepath.Dir(x.JobsDir)
	}
	client, err := memory.Resolve(filepath.Join(configDir, "config.toml"))
	if err != nil {
		return result, fmt.Errorf("memory unavailable: %w", err)
	}
	note := "agentcron/" + j.Name
	// Include the resolved binary/root in the cache so changing KBs bootstraps again.
	key := client.Binary + "\n" + client.Root + "\n" + note
	cached, err := x.DB.MemoryNote(ctx, j.Name)
	if err == nil && cached != key {
		err = client.Ensure(ctx, j.Name, j.Schedule, j.Runner)
		if err == nil {
			err = x.DB.SetMemoryNote(ctx, j.Name, key)
		}
	}
	if err == nil {
		result.flags, err = client.Inject(j.Runner, dir, j.Args)
	}
	if err != nil {
		return runMemory{}, fmt.Errorf("memory unavailable: %w", err)
	}
	result.preamble = memory.Preamble(j.Name, j.Runner == "claude" && memory.HasMCPConfig(j.Args))
	result.env = []string{"AGENTCRON_MEMORY_NOTE=" + note}
	if client.Root != "" {
		result.env = append(result.env, "MDKB_ROOT="+client.Root)
	}
	return result, nil
}

func memoryArgv(j jobs.Job, r store.Run, prompt string, flags []string) []string {
	j.Args = append(append([]string{}, flags...), j.Args...)
	return Argv(j, r.Worktree, prompt)
}

func runError(existing string, cause error) string {
	if existing != "" {
		return existing + "; " + cause.Error()
	}
	return cause.Error()
}
