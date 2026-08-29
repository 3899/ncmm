//go:build windows

package ncmm

import (
	"bufio"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

func stdinIsTerminal(file *os.File) bool {
	var mode uint32
	return windows.GetConsoleMode(windows.Handle(file.Fd()), &mode) == nil
}

func readTerminalPassword(file *os.File) (string, error) {
	handle := windows.Handle(file.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return "", err
	}
	if err := windows.SetConsoleMode(handle, mode&^windows.ENABLE_ECHO_INPUT); err != nil {
		return "", err
	}
	defer windows.SetConsoleMode(handle, mode)
	line, err := bufio.NewReader(file).ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}
