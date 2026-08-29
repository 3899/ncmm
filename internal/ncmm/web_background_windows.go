//go:build windows

package ncmm

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const windowsCreateNoWindow = 0x08000000

func startWebBackground(listen string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}
	readyURL, err := backgroundReadyURL(listen)
	if err != nil {
		return err
	}

	cmd := exec.Command(executable, withoutBackgroundFlag(os.Args[1:])...)
	cmd.Dir = workingDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | windowsCreateNoWindow,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start background WebUI: %w", err)
	}

	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		response, requestErr := client.Get(readyURL)
		if requestErr == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusInternalServerError {
				_ = cmd.Process.Release()
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}

	_ = cmd.Process.Kill()
	_ = cmd.Process.Release()
	return fmt.Errorf("background WebUI did not become ready at %s", readyURL)
}

func withoutBackgroundFlag(args []string) []string {
	result := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--background" || strings.HasPrefix(arg, "--background=") {
			continue
		}
		result = append(result, arg)
	}
	return result
}

func backgroundReadyURL(listen string) (string, error) {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", fmt.Errorf("invalid WebUI listen address %q: %w", listen, err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return (&url.URL{Scheme: "http", Host: net.JoinHostPort(host, port), Path: "/api/v1/auth/status"}).String(), nil
}
