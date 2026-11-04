package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/ContractorKeith/agentcron/internal/clock"
)

// LogTail reads at most 64 KiB from the end of a ledger run's output.
func LogTail(ctx context.Context, opts Options, c clock.Clock, id string, stderr bool) (string, error) {
	s, err := openJobs(ctx, opts, c, true)
	if err != nil {
		return "", err
	}
	defer s.db.Close()
	r, err := s.db.GetRun(ctx, id)
	if err != nil {
		return "", err
	}
	name := "stdout.log"
	if stderr {
		name = "stderr.log"
	}
	f, err := os.Open(filepath.Join(s.config.DataDir, "runs", r.ID, name))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	const limit = 64 * 1024
	if _, err = f.Seek(max(0, st.Size()-limit), io.SeekStart); err != nil {
		return "", err
	}
	b, err := io.ReadAll(io.LimitReader(f, limit))
	return string(b), err
}
