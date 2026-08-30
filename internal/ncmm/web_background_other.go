//go:build !windows

package ncmm

import "fmt"

func startWebBackground(string, string) error {
	return fmt.Errorf("--background is only supported on Windows")
}
