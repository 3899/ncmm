//go:build !windows

package ncmm

import "syscall"

func stopWebProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}
