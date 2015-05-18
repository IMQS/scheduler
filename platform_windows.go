package scheduler

import (
	"os/exec"
	"strconv"
)

// We need a function for killing a process and it's offspring, leaving no traces after the attempt.
// This is especially necessary for sufficiently killing all git subprocesses.
func killProcessTree(pid int) bool {
	killcmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	err := killcmd.Run()
	if err != nil {
		return false
	}
	return true
}
