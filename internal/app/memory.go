package app

import (
	"context"

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
	if _, err := jobs.ReadForRun(s.config.JobsDir(), name); err != nil {
		return "", &Error{Usage, err}
	}
	client, err := memory.Resolve(s.config.File())
	if err != nil {
		return "", err
	}
	out, err := client.Run(ctx, "cat", "agentcron/"+name)
	return string(out), err
}
