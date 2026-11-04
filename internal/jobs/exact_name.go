package jobs

import (
	"os"
)

func exactEntry(entries []os.DirEntry, name string) bool {
	for _, entry := range entries {
		if entry.Name() == name {
			return true
		}
	}
	return false
}
func requireExact(dir, name string) error {
	exists, err := Exists(dir, name)
	if err != nil {
		return err
	}
	if !exists {
		return os.ErrNotExist
	}
	return nil
}
