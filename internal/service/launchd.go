package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/ContractorKeith/agentcron/internal/clock"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (o Options) pause(ctx context.Context, d time.Duration) error {
	c := o.Clock
	if c == nil {
		c = clock.Real{}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.After(d):
		return nil
	}
}
func waitUnloaded(ctx context.Context, o Options, cmd Commander) error {
	c := o.Clock
	if c == nil {
		c = clock.Real{}
	}
	deadline := c.Now().Add(10 * time.Second)
	for {
		remaining := deadline.Sub(c.Now())
		if remaining <= 0 {
			return fmt.Errorf("launchd unload did not finish within 10s")
		}
		pollCtx, cancel := context.WithTimeout(ctx, remaining)
		out, err := cmd.Run(pollCtx, "launchctl", "print", o.domain()+"/com.contractorkeith.agentcron")
		cancel()
		if err != nil {
			if notLoaded(out + " " + err.Error()) {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("launchd unload: %w", err)
		}
		if err := o.pause(ctx, min(100*time.Millisecond, deadline.Sub(c.Now()))); err != nil {
			return err
		}
	}
}
func bootstrapFive(out string, err error) bool {
	var code interface{ ExitCode() int }
	return errors.As(err, &code) && code.ExitCode() == 5 || strings.Contains(strings.ToLower(out+" "+err.Error()), "bootstrap failed: 5")
}
func installLaunchd(ctx context.Context, o Options, d Definition, cmd Commander) (Definition, error) {
	old, err := os.ReadFile(d.Path)
	existed := err == nil
	if err != nil && !os.IsNotExist(err) {
		return d, err
	}
	out, err := cmd.Run(ctx, "launchctl", "bootout", o.domain()+"/com.contractorkeith.agentcron")
	if err != nil && !notLoaded(out+" "+err.Error()) {
		return d, err
	}
	if err = waitUnloaded(ctx, o, cmd); err != nil {
		return d, err
	}
	if err = os.MkdirAll(filepath.Dir(d.Path), 0700); err != nil {
		return d, err
	}
	if err = os.MkdirAll(o.DataDir, 0700); err != nil {
		return d, err
	}
	restore := func(cause error) (Definition, error) {
		var restoreErr error
		if existed {
			restoreErr = os.WriteFile(d.Path, old, 0600)
		} else {
			restoreErr = remove(d.Path)
		}
		return d, errors.Join(cause, restoreErr)
	}
	if err = os.WriteFile(d.Path, []byte(d.Unit), 0600); err != nil {
		return restore(err)
	}
	for attempt := 0; attempt <= 3; attempt++ {
		out, err = cmd.Run(ctx, "launchctl", "bootstrap", o.domain(), o.file())
		if err == nil {
			return d, nil
		}
		if attempt == 3 || !bootstrapFive(out, err) {
			return restore(err)
		}
		if err = o.pause(ctx, 500*time.Millisecond); err != nil {
			return restore(err)
		}
	}
	return d, nil
}
