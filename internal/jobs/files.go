package jobs

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Entry struct {
	Name  string `json:"name"`
	Job   *Job   `json:"job,omitempty"`
	Error string `json:"error,omitempty"`
}

func Read(dir, name string) (Job, error) {
	if err := ValidateName(name); err != nil {
		return Job{}, err
	}
	if err := requireExact(dir, name); err != nil {
		return Job{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, name+".toml"))
	if err != nil {
		return Job{}, err
	}
	return Parse(dir, name, data)
}

// ReadForRun defers prompt-file availability checks to preflight, so failures
// receive a blocked_config ledger row rather than disappearing as usage errors.
func ReadForRun(dir, name string) (Job, error) {
	if err := ValidateName(name); err != nil {
		return Job{}, err
	}
	if err := requireExact(dir, name); err != nil {
		return Job{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, name+".toml"))
	if err != nil {
		return Job{}, err
	}
	return parse(dir, name, data, false)
}
func ReadPrompt(dir string, j Job) (string, error) {
	if err := j.Validate(dir); err != nil {
		return "", err
	}
	if j.PromptFile == "" {
		return j.Prompt, nil
	}
	b, err := os.ReadFile(filepath.Join(dir, j.PromptFile))
	return string(b), err
}

func List(dir string) ([]Entry, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := []Entry{}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".toml") {
			continue
		}
		name := strings.TrimSuffix(file.Name(), ".toml")
		j, err := Read(dir, name)
		entry := Entry{Name: name}
		if err != nil {
			entry.Error = err.Error()
		} else {
			entry.Job = &j
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func Encode(j Job) (string, error) {
	var b bytes.Buffer
	err := toml.NewEncoder(&b).Encode(j)
	return b.String(), err
}

func Exists(dir, name string) (bool, error) {
	if err := ValidateName(name); err != nil {
		return false, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return exactEntry(entries, name+".toml"), nil
}

// Write uses a same-directory temporary file; readers only see complete definitions.
func Write(dir string, j Job) error { return publish(dir, j, os.Rename) }

// Create publishes a complete definition only if its name does not already exist.
func Create(dir string, j Job) error { return publish(dir, j, os.Link) }

func publish(dir string, j Job, commit func(string, string) error) error {
	if err := j.Validate(dir); err != nil {
		return err
	}
	data, err := Encode(j)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".job-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.WriteString(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = commit(f.Name(), filepath.Join(dir, j.Name+".toml")); err != nil {
		return fmt.Errorf("write job: %w", err)
	}
	return nil
}

func Remove(dir, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if err := requireExact(dir, name); err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, name+".toml"))
}
