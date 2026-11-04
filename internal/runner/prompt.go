package runner

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func Preamble(r store.Run, last *store.Run, data string) string {
	state, output := "none", "none"
	if last != nil {
		state = last.State
		output = filepath.Join(data, "runs", last.ID, "stdout.log")
	}
	return fmt.Sprintf("Job: %s\nRun: %s\nTrigger: %s\nScheduled for: %s\nLast run state: %s\nLast output: %s\n%s\n", r.Job, r.ID, r.Trigger, r.ScheduledFor.UTC().Format(time.RFC3339), state, output, memoryPreamble())
}

// memoryPreamble is the M4 hook for the job's memory note.
func memoryPreamble() string { return "" }
func Argv(j jobs.Job, workdir, prompt string) []string {
	switch j.Runner {
	case "claude":
		return append([]string{"-p", prompt, "--output-format", "text"}, j.Args...)
	case "codex":
		return append(append([]string{"exec", "-C", workdir}, j.Args...), prompt)
	default:
		return append(append([]string{}, j.Command[1:]...), j.Args...)
	}
}
