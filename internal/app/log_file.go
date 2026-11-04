package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// OpenRoot confines path resolution even if a run-directory symlink is swapped.
func openRegularLog(runsDir, id, stream string) (*os.File, error) {
	root, err := os.OpenRoot(runsDir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	dir, err := root.OpenFile(id, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	fd, err := unix.Openat(int(dir.Fd()), stream, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return nil, fmt.Errorf("log is not a regular file")
		}
		return nil, err
	}
	f := os.NewFile(uintptr(fd), filepath.Join(runsDir, id, stream))
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("log is not a regular file")
	}
	return f, nil
}
