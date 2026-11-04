// Package app implements use cases shared by agentcron's interfaces.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/ContractorKeith/agentcron/internal/config"
	"github.com/ContractorKeith/agentcron/internal/store"
)

type Options struct {
	ConfigDir string
	DataDir   string
	// ReadOnly evaluates pinned-time previews without changing persisted job state.
	ReadOnly bool
}

func (o Options) configOptions() config.Options {
	return config.Options{ConfigDir: o.ConfigDir, DataDir: o.DataDir}
}

// Init creates directories, default settings and the ledger, preserving existing settings.
func Init(ctx context.Context, opts Options) error {
	c, err := config.Load(opts.configOptions())
	if err != nil {
		return err
	}
	for _, dir := range []string{c.ConfigDir, c.JobsDir(), c.DataDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	file, err := os.OpenFile(c.File(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		_, writeErr := file.WriteString(config.Defaults)
		closeErr := file.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrExist) {
		return err
	}
	db, err := store.Open(ctx, c.Database())
	if err != nil {
		return err
	}
	return db.Close()
}

type Health struct {
	ConfigDir     bool   `json:"config_dir"`
	DataDir       bool   `json:"data_dir"`
	SchemaVersion int    `json:"schema_version"`
	Git           bool   `json:"git"`
	Claude        bool   `json:"claude"`
	Codex         bool   `json:"codex"`
	MDKB          bool   `json:"mdkb"`
	ConfigPath    string `json:"-"`
	DataPath      string `json:"-"`
}

// Doctor inspects paths and executable presence without creating a missing ledger.
func Doctor(ctx context.Context, opts Options) (Health, error) {
	c, err := config.Load(opts.configOptions())
	if err != nil {
		return Health{}, err
	}
	h := Health{
		ConfigDir: isDir(c.ConfigDir), DataDir: isDir(c.DataDir),
		ConfigPath: c.ConfigDir, DataPath: c.DataDir,
		Git: onPath("git"), Claude: onPath("claude"), Codex: onPath("codex"), MDKB: onPath("mdkb"),
	}
	if _, err := os.Stat(c.Database()); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return h, &Error{NotInitialized, fmt.Errorf("data directory is not initialised; run agentcron init")}
		}
		return h, err
	}
	db, err := store.OpenExisting(ctx, c.Database())
	if err != nil {
		return h, err
	}
	defer db.Close()
	h.SchemaVersion, err = db.SchemaVersion(ctx)
	return h, err
}
func isDir(path string) bool  { info, err := os.Stat(path); return err == nil && info.IsDir() }
func onPath(name string) bool { _, err := exec.LookPath(name); return err == nil }
