// +build !windows

package scheduler

func RunAsService(handler func()) bool {
	return false
}
