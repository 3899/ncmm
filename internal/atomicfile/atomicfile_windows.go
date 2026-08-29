//go:build windows

package atomicfile

import (
	"errors"
	"time"

	"golang.org/x/sys/windows"
)

func replace(source, destination string) error {
	sourcePtr, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	destinationPtr, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		err = windows.MoveFileEx(
			sourcePtr,
			destinationPtr,
			windows.MOVEFILE_REPLACE_EXISTING,
		)
		if err == nil {
			return nil
		}
		if (!errors.Is(err, windows.ERROR_SHARING_VIOLATION) && !errors.Is(err, windows.ERROR_ACCESS_DENIED)) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func syncDir(string) error {
	return nil
}
