//go:build windows

package ncmm

import "os"

func stopWebProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}
