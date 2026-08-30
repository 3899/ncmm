//go:build windows

package ncmm

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/3899/ncmm/internal/webui"
)

const windowsCreateNoWindow = 0x08000000

func startWebBackground(listen, home string) error {
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
	output := &backgroundOutput{limit: 64 << 10}
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start background WebUI: %w", err)
	}
	exit := make(chan error, 1)
	go func() { exit <- cmd.Wait() }()

	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case processErr := <-exit:
			message := strings.TrimSpace(output.String())
			if message == "" && processErr != nil {
				message = processErr.Error()
			}
			return fmt.Errorf("background WebUI exited before readiness: %s", message)
		default:
		}
		info, running, inspectErr := webui.InspectInstance(home)
		if inspectErr == nil && running && info.PID == cmd.Process.Pid && info.InstanceID != "" {
			if backgroundInstanceReady(client, readyURL, info.InstanceID) {
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}

	_ = cmd.Process.Kill()
	select {
	case <-exit:
	case <-time.After(5 * time.Second):
	}
	message := strings.TrimSpace(output.String())
	if message != "" {
		return fmt.Errorf("background WebUI did not become ready at %s: %s", readyURL, message)
	}
	return fmt.Errorf("background WebUI did not become ready at %s", readyURL)
}

func backgroundInstanceReady(client *http.Client, readyURL, expectedInstanceID string) bool {
	response, err := client.Get(readyURL)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, response.Body)
		return false
	}
	var payload struct {
		InstanceID string `json:"instanceId"`
	}
	return json.NewDecoder(io.LimitReader(response.Body, 8<<10)).Decode(&payload) == nil && payload.InstanceID == expectedInstanceID
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
	return (&url.URL{Scheme: "http", Host: net.JoinHostPort(host, port), Path: "/api/v1/instance"}).String(), nil
}

type backgroundOutput struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func (w *backgroundOutput) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	written := len(data)
	w.data = append(w.data, data...)
	if overflow := len(w.data) - w.limit; overflow > 0 {
		copy(w.data, w.data[overflow:])
		w.data = w.data[:w.limit]
	}
	return written, nil
}

func (w *backgroundOutput) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return string(w.data)
}
