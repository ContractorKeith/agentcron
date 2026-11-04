package app

import (
	"context"
	"github.com/ContractorKeith/agentcron/internal/clock"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestZeroRetentionIsUsageError(t *testing.T) {
	opts := isolated(t)
	ctx := context.Background()
	if err := Init(ctx, opts); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opts.ConfigDir, "config.toml"), []byte("keep_runs = 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	err := Schedule(ctx, opts, clock.NewFake(time.Time{}), false, false, nil, nil)
	if Code(err) != Usage || !strings.Contains(err.Error(), "keep_runs") {
		t.Fatalf("code=%d error=%v", Code(err), err)
	}
}
