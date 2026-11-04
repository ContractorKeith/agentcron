package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type instantClock struct{ elapsed time.Duration }

func (c *instantClock) Now() time.Time {
	return time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC).Add(c.elapsed)
}
func (c *instantClock) After(d time.Duration) <-chan time.Time {
	c.elapsed += d
	ch := make(chan time.Time, 1)
	ch <- c.Now()
	return ch
}

type exitFive struct{}

func (exitFive) Error() string { return "Bootstrap failed: 5: Input/output error" }
func (exitFive) ExitCode() int { return 5 }

type launchdFake struct {
	polls, failures, prints, boots int
	path                           string
	old                            []byte
	checkOld                       bool
	bootoutErr                     error
}

func (f *launchdFake) Run(_ context.Context, _ string, args ...string) (string, error) {
	switch args[0] {
	case "bootout":
		if f.checkOld {
			raw, _ := os.ReadFile(f.path)
			if string(raw) != string(f.old) {
				return "", errors.New("plist changed before bootout")
			}
		}
		return "", f.bootoutErr
	case "print":
		f.prints++
		if f.polls > 0 {
			f.polls--
			return "state = running", nil
		}
		return "Could not find service", errors.New("not found")
	case "bootstrap":
		f.boots++
		if f.polls > 0 || f.failures > 0 {
			f.failures--
			return "Bootstrap failed: 5", exitFive{}
		}
		return "", nil
	}
	return "", nil
}
func TestLaunchdWaitRetryAndRollback(t *testing.T) {
	for _, kind := range []string{"delayed-bootout", "retry", "exhausted", "bootout-failed", "poll-timeout"} {
		t.Run(kind, func(t *testing.T) {
			c := &instantClock{}
			o := Options{OS: "darwin", Home: t.TempDir(), DataDir: t.TempDir(), Binary: "/new", Clock: c}
			old := []byte("previous plist")
			if err := os.MkdirAll(filepath.Dir(o.file()), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(o.file(), old, 0600); err != nil {
				t.Fatal(err)
			}
			f := &launchdFake{path: o.file(), old: old, checkOld: true}
			switch kind {
			case "delayed-bootout":
				f.polls = 3
			case "retry":
				f.failures = 1
			case "exhausted":
				f.failures = 10
			case "bootout-failed":
				f.bootoutErr = errors.New("permission denied")
			case "poll-timeout":
				f.polls = 1000
			}
			_, err := Apply(t.Context(), o, "install", false, f)
			bad := kind == "exhausted" || kind == "bootout-failed" || kind == "poll-timeout"
			if (err != nil) != bad {
				t.Fatalf("err=%v prints=%d boots=%d", err, f.prints, f.boots)
			}
			raw, _ := os.ReadFile(o.file())
			if bad && string(raw) != string(old) {
				t.Fatal("previous plist not restored")
			}
			if kind == "delayed-bootout" && (f.prints != 4 || c.elapsed != 300*time.Millisecond) {
				t.Fatalf("prints=%d elapsed=%v", f.prints, c.elapsed)
			}
			if kind == "retry" && (f.boots != 2 || c.elapsed != 500*time.Millisecond) {
				t.Fatalf("boots=%d elapsed=%v", f.boots, c.elapsed)
			}
			if kind == "exhausted" && f.boots != 4 {
				t.Fatal(f.boots)
			}
			if kind == "poll-timeout" && (c.elapsed != 10*time.Second || !strings.Contains(err.Error(), "unload")) {
				t.Fatal(c.elapsed, err)
			}
		})
	}
}
