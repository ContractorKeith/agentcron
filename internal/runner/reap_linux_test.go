package runner

import (
	"syscall"
	"time"
)

func init() { _, _, _ = syscall.Syscall6(syscall.SYS_PRCTL, 36, 1, 0, 0, 0, 0) } // Adopt orphan test grandchildren.
func reapDescendant(pid int) {
	for range 100 {
		got, err := syscall.Wait4(pid, nil, syscall.WNOHANG, nil)
		if got == pid || err != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
