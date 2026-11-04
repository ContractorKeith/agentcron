package memory

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func (c Client) Ensure(ctx context.Context, job, schedule, runner string) error {
	_, err := c.Run(ctx, "cat", "agentcron/"+job)
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		return err
	}
	// mdkb new caps slugs at 40 characters, collapses dashes and trims their
	// suffix. Refuse an incompatible title rather than create a different note.
	if len(job) > 40 || strings.Contains(job, "--") || strings.HasSuffix(job, "-") {
		return fmt.Errorf("mdkb cannot create the exact note agentcron/%s from this job name", job)
	}
	body := fmt.Sprintf("# %s\n\nJob: %s\nSchedule: %s\nRunner: %s\nCreated by agentcron.\n", job, job, schedule, runner)
	_, err = c.Run(ctx, "new", job, "--dir", "agentcron", "--body", body)
	return err
}
