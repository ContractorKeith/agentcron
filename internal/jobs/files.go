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
	data, err := os.ReadFile(filepath.Join(dir, name+".toml"))
	if err != nil {
		return Job{}, err
	}
	return Parse(dir, name, data)
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
	_, err := os.Lstat(filepath.Join(dir, name+".toml"))
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// Write uses a same-directory temporary file; readers only see complete definitions.
func Write(dir string, j Job) error {
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
	if err = os.Rename(f.Name(), filepath.Join(dir, j.Name+".toml")); err != nil {
		return fmt.Errorf("write job: %w", err)
	}
	return nil
}

func Remove(dir, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, name+".toml"))
}
