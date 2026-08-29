//go:build !windows && !linux

package ncmm

import (
	"fmt"
	"os"
)

func stdinIsTerminal(*os.File) bool {
	return false
}

func readTerminalPassword(*os.File) (string, error) {
	return "", fmt.Errorf("interactive password input is not supported on this platform")
}
