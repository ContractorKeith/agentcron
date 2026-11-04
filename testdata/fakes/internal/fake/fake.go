// Package fake implements the test runners; no real agent or KB is invoked.
package fake

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type Record struct {
	Argv  []string          `json:"argv"`
	Cwd   string            `json:"cwd"`
	Env   map[string]string `json:"env"`
	Stdin string            `json:"stdin"`
}

func Main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	mode := os.Getenv("FAKE_MODE")
	if mode == "ignore-term" {
		signal.Ignore(syscall.SIGTERM)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	var input []byte
	if os.Getenv("FAKE_STDIN") == "1" {
		input, err = io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
	}
	env := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "AGENTCRON_") || key == "MDKB_ROOT" {
			env[key] = value
		}
	}
	if path := os.Getenv("FAKE_RECORD"); path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		encodeErr := json.NewEncoder(f).Encode(Record{os.Args, cwd, env, string(input)})
		closeErr := f.Close()
		if encodeErr != nil {
			return encodeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	duration := 60 * time.Second
	if value := os.Getenv("FAKE_SLEEP"); value != "" {
		duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
	}
	switch mode {
	case "", "ok":
		fmt.Println("DONE")
	case "fail":
		return fmt.Errorf("fake failure")
	case "sleep", "ignore-term":
		time.Sleep(duration)
	case "noisy":
		chunk := strings.Repeat("x", 1024)
		for range 10 * 1024 {
			stream := io.Writer(os.Stdout)
			switch os.Getenv("FAKE_NOISY_STREAM") {
			case "stderr":
				stream = os.Stderr
			case "both":
				stream = io.MultiWriter(os.Stdout, os.Stderr)
			}
			if _, err := io.WriteString(stream, chunk); err != nil {
				return err
			}
		}
	case "dirty":
		return os.WriteFile("result.txt", []byte("DONE\n"), 0600)
	case "spawn-child":
		terms := make(chan os.Signal, 1)
		signal.Notify(terms, syscall.SIGTERM)
		defer signal.Stop(terms)
		child := exec.Command(os.Args[0])
		child.Env = childEnv()
		if err := child.Start(); err != nil {
			return err
		}
		// The runner signals the process group; reap the child before returning normally.
		select {
		case <-time.After(duration):
		case <-terms:
		}
		_ = child.Process.Signal(syscall.SIGTERM)
		return child.Wait()
	default:
		return fmt.Errorf("unknown FAKE_MODE %q", mode)
	}
	return nil
}

func childEnv() []string {
	env := os.Environ()
	// Replace, rather than duplicate, inherited values.
	result := make([]string, 0, len(env))
	for _, entry := range env {
		if !strings.HasPrefix(entry, "FAKE_MODE=") && !strings.HasPrefix(entry, "FAKE_RECORD=") {
			result = append(result, entry)
		}
	}
	return append(result, "FAKE_MODE=sleep", "FAKE_RECORD=")
}
