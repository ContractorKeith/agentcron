package app

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/memory"
)

// Memory prints an existing job's note, without bootstrapping or modifying the KB.
func Memory(ctx context.Context, opts Options, name string) (string, error) {
	s, err := openJobs(ctx, opts, clock.Real{}, true)
	if err != nil {
		return "", err
	}
	defer s.db.Close()
	j, err := jobs.ReadForRun(s.config.JobsDir(), name)
	if err != nil {
		return "", &Error{Usage, err}
	}
	if !j.Memory {
		return "", &Error{Usage, fmt.Errorf("memory is off for this job")}
	}
	client, err := memory.Resolve(s.config.MDKBPath, s.config.MDKBRoot)
	if err != nil {
		return "", err
	}
	out, err := client.Run(ctx, "cat", "agentcron/"+name)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 2 {
		return "", &Error{Usage, fmt.Errorf("no memory note yet for %s", name)}
	}
	return string(out), err
}
