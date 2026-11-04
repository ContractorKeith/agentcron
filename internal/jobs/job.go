// Package jobs exclusively owns job definitions and their files.
package jobs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/ContractorKeith/agentcron/internal/schedule"
)

type Job struct {
	Name       string   `toml:"-" json:"name"`
	Schedule   string   `toml:"schedule" json:"schedule"`
	Timezone   string   `toml:"timezone" json:"timezone"`
	Runner     string   `toml:"runner" json:"runner"`
	Prompt     string   `toml:"prompt,omitempty" json:"prompt,omitempty"`
	PromptFile string   `toml:"prompt_file,omitempty" json:"prompt_file,omitempty"`
	Repo       string   `toml:"repo,omitempty" json:"repo,omitempty"`
	Base       string   `toml:"base,omitempty" json:"base,omitempty"`
	Args       []string `toml:"args,omitempty" json:"args"`
	Command    []string `toml:"command,omitempty" json:"command,omitempty"`
	Timeout    string   `toml:"timeout" json:"timeout"`
	Expect     string   `toml:"expect,omitempty" json:"expect,omitempty"`
	Memory     bool     `toml:"memory" json:"memory"`
	Catchup    string   `toml:"catchup" json:"catchup"`
	CreatedBy  string   `toml:"created_by" json:"created_by"`
}

func Defaults(name string) Job {
	return Job{Name: name, Timeout: "30m", Memory: true, Catchup: "one", CreatedBy: "cli", Args: []string{}}
}

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

func ValidateName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("name: must match %s", validName)
	}
	return nil
}

func (j *Job) Validate(dir string) error {
	if err := ValidateName(j.Name); err != nil {
		return err
	}
	if _, err := schedule.Parse(j.Schedule); err != nil {
		return err
	}
	if j.Runner != "claude" && j.Runner != "codex" && j.Runner != "command" {
		return fmt.Errorf("runner: must be claude, codex or command")
	}
	if (j.Prompt == "") == (j.PromptFile == "") {
		return fmt.Errorf("prompt/prompt_file: exactly one is required")
	}
	if j.Runner == "command" {
		if len(j.Command) == 0 || j.Command[0] == "" {
			return fmt.Errorf("command: runner command requires argv")
		}
	} else if j.Command != nil {
		return fmt.Errorf("command: only valid for runner command")
	}
	if j.PromptFile != "" {
		if filepath.IsAbs(j.PromptFile) {
			return fmt.Errorf("prompt_file: must be relative to jobs directory")
		}
		root, err := filepath.EvalSymlinks(dir)
		if err != nil {
			return fmt.Errorf("prompt_file: %w", err)
		}
		target, err := filepath.EvalSymlinks(filepath.Join(dir, j.PromptFile))
		if err != nil {
			return fmt.Errorf("prompt_file: %w", err)
		}
		rel, err := filepath.Rel(root, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("prompt_file: escapes jobs directory")
		}
		info, err := os.Stat(target)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("prompt_file: must be a regular file")
		}
	}
	d, err := time.ParseDuration(j.Timeout)
	if err != nil || d <= 0 {
		return fmt.Errorf("timeout: must be a positive duration")
	}
	if _, err := regexp.Compile(j.Expect); err != nil {
		return fmt.Errorf("expect: %w", err)
	}
	if j.Timezone != "" {
		if _, err := time.LoadLocation(j.Timezone); err != nil {
			return fmt.Errorf("timezone: %w", err)
		}
	}
	if j.Catchup != "one" && j.Catchup != "none" {
		return fmt.Errorf("catchup: must be one or none")
	}
	if j.Base != "" && j.Repo == "" {
		return fmt.Errorf("base: requires repo")
	}
	if j.CreatedBy != "cli" && j.CreatedBy != "mcp" {
		return fmt.Errorf("created_by: must be cli or mcp")
	}
	if j.Repo == "~" || strings.HasPrefix(j.Repo, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("repo: %w", err)
		}
		j.Repo = filepath.Join(home, strings.TrimPrefix(j.Repo, "~"))
	}
	return nil
}

func Parse(dir, name string, data []byte) (Job, error) {
	j := Defaults(name)
	meta, err := toml.Decode(string(data), &j)
	if err != nil {
		return j, err
	}
	if unknown := meta.Undecoded(); len(unknown) > 0 {
		return j, fmt.Errorf("unknown field: %s", unknown[0])
	}
	err = j.Validate(dir)
	return j, err
}
