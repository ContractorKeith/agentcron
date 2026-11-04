package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ContractorKeith/agentcron/internal/clock"
)

type RunOutput struct {
	ID        string `json:"id"`
	Stream    string `json:"stream"`
	Output    string `json:"output"`
	Truncated bool   `json:"truncated"`
}

// TailOutput reads at most tailBytes bytes, even while a runner appends output.
func TailOutput(ctx context.Context, opts Options, c clock.Clock, id, stream string, tailBytes int) (RunOutput, error) {
	out := RunOutput{ID: id, Stream: stream}
	if stream != "stdout" && stream != "stderr" {
		return out, fmt.Errorf("stream must be stdout or stderr")
	}
	if tailBytes < 1 || tailBytes > 256*1024 {
		return out, fmt.Errorf("tail_bytes must be 1–262144")
	}
	s, err := openJobs(ctx, opts, c, true)
	if err != nil {
		return out, err
	}
	defer s.db.Close()
	r, err := s.db.GetRun(ctx, id)
	if err != nil {
		return out, err
	}
	f, err := os.Open(filepath.Join(s.config.DataDir, "runs", r.ID, stream+".log"))
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return out, err
	}
	start := max(int64(0), info.Size()-int64(tailBytes))
	out.Truncated = start > 0
	if _, err = f.Seek(start, io.SeekStart); err != nil {
		return out, err
	}
	raw, err := io.ReadAll(io.LimitReader(f, int64(tailBytes)))
	out.Output = string(raw)
	return out, err
}
