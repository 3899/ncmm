//go:build !windows

package atomicfile

import (
	"os"
)

func replace(source, destination string) error {
	return os.Rename(source, destination)
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
