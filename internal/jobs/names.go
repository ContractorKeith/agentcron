package jobs

import (
	"fmt"
	"os"
	"strings"
)

type missingJobError struct{ name string }

func (e missingJobError) Error() string { return fmt.Sprintf("unknown job %q", e.name) }
func (e missingJobError) Unwrap() error { return os.ErrNotExist }

// CheckAvailable rejects both an exact collision and a case variant before a
// dry run or a real create, consistently on case-sensitive and insensitive disks.
func CheckAvailable(dir, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	entries, err := readJobDir(dir)
	if err != nil {
		return err
	}
	if exactEntry(entries, name+".toml") {
		return os.ErrExist
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), name+".toml") {
			return fmt.Errorf("a job file differing only in case exists: %s", entry.Name())
		}
	}
	return nil
}
