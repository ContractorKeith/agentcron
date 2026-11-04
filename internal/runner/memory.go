package runner

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ContractorKeith/agentcron/internal/config"
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
	cfg, err := config.Load(config.Options{ConfigDir: configDir, DataDir: x.DataDir})
	if err != nil {
		return result, fmt.Errorf("memory unavailable: %w", err)
	}
	client, err := memory.Resolve(cfg.MDKBPath, cfg.MDKBRoot)
	if err != nil {
		return result, fmt.Errorf("memory unavailable: %w", err)
	}
	note := "agentcron/" + j.Name
	// Record the last verified binary/root and note; verification still runs every time.
	key := client.Binary + "\n" + client.Root + "\n" + note
	err = client.Ensure(ctx, j.Name, j.Schedule, j.Runner)
	if err == nil {
		err = x.DB.SetMemoryNote(ctx, j.Name, key)
	}
	if err == nil {
		result.flags, err = client.Inject(j.Runner, dir, j.Args)
	}
	if err != nil {
		return runMemory{}, fmt.Errorf("memory unavailable: %w", err)
	}
	result.preamble = memory.Preamble(j.Name, memory.Override(j.Runner, j.Args))
	result.env = []string{"AGENTCRON_MEMORY_NOTE=" + note}
	if client.Root != "" {
		result.env = append(result.env, "MDKB_ROOT="+client.Root)
	}
	return result, nil
}

func memoryArgv(j jobs.Job, r store.Run, prompt string, flags []string) []string {
	flags, args := memoryPermissions(flags, j.Args)
	j.Args = append(append([]string{}, flags...), args...)
	return Argv(j, r.Worktree, prompt)
}

func runError(existing string, cause error) string {
	if existing != "" {
		return existing + "; " + cause.Error()
	}
	return cause.Error()
}
