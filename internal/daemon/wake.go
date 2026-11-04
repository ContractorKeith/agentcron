package daemon

import "syscall"

// Wake validates the recorded process identity without competing for its lease.
func Wake(data string) bool {
	pid := Holder(data)
	return pid > 0 && syscall.Kill(pid, syscall.SIGHUP) == nil
}
