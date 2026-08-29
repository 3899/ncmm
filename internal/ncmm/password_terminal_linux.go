//go:build linux

package ncmm

import (
	"bufio"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func stdinIsTerminal(file *os.File) bool {
	_, err := unix.IoctlGetTermios(int(file.Fd()), unix.TCGETS)
	return err == nil
}

func readTerminalPassword(file *os.File) (string, error) {
	fd := int(file.Fd())
	original, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return "", err
	}
	updated := *original
	updated.Lflag &^= unix.ECHO
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &updated); err != nil {
		return "", err
	}
	defer unix.IoctlSetTermios(fd, unix.TCSETS, original)
	line, err := bufio.NewReader(file).ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}
