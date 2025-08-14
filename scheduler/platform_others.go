//go:build !windows

package scheduler

import "syscall"

func killProcessTree(pid int) bool {
	err := syscall.Kill(-pid, syscall.SIGKILL)
	if err != nil {
		return false
	}
	return true
}
