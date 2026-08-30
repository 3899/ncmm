package ncmm

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/3899/ncmm/internal/filelock"
	"github.com/3899/ncmm/internal/webui"
)

const webInstanceHelperMarker = "--web-instance-helper"

func TestWebInstanceHelperProcess(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == webInstanceHelperMarker {
			index = i
			break
		}
	}
	if index < 0 || index+3 >= len(os.Args) {
		return
	}
	home, instanceID, ready := os.Args[index+1], os.Args[index+2], os.Args[index+3]
	lock, err := filelock.TryAcquire(filepath.Join(home, webui.InstanceLockFilename))
	if err != nil {
		os.Exit(2)
	}
	metadata := map[string]any{
		"pid": os.Getpid(), "startedAt": time.Now().UTC(), "listen": "127.0.0.1:3899",
		"version": "1.2.0", "instanceId": instanceID,
	}
	data, _ := json.Marshal(metadata)
	if err := lock.Write(append(data, '\n')); err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(ready, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		os.Exit(2)
	}
	for {
		time.Sleep(time.Hour)
	}
}

func TestWebStatusAndStopTargetOnlySelectedHome(t *testing.T) {
	firstHome := t.TempDir()
	secondHome := t.TempDir()
	first := startWebInstanceHelper(t, firstHome, "first-instance")
	second := startWebInstanceHelper(t, secondHome, "second-instance")
	defer func() {
		_ = first.Process.Kill()
		_ = second.Process.Kill()
		_ = first.Wait()
		_ = second.Wait()
	}()

	output := new(bytes.Buffer)
	root := New()
	root.cmd.SetOut(output)
	root.cmd.SetErr(output)
	root.cmd.SetArgs([]string{"--home", firstHome, "web", "status"})
	if err := root.cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "first-instance") || !strings.Contains(output.String(), strconv.Itoa(first.Process.Pid)) {
		t.Fatalf("status output = %q", output.String())
	}

	output.Reset()
	root = New()
	root.cmd.SetOut(output)
	root.cmd.SetErr(output)
	root.cmd.SetArgs([]string{"--home", firstHome, "web", "stop"})
	if err := root.cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "first-instance") {
		t.Fatalf("stop output = %q", output.String())
	}
	if _, running, err := webui.InspectInstance(firstHome); err != nil || running {
		t.Fatalf("first instance after stop = running %v, err %v", running, err)
	}
	if info, running, err := webui.InspectInstance(secondHome); err != nil || !running || info.InstanceID != "second-instance" {
		t.Fatalf("unrelated instance after stop = %+v, running %v, err %v", info, running, err)
	}
}

func startWebInstanceHelper(t *testing.T, home, instanceID string) *exec.Cmd {
	t.Helper()
	ready := filepath.Join(home, "ready")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWebInstanceHelperProcess$", "--", webInstanceHelperMarker, home, instanceID, ready)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(ready); err == nil {
			return cmd
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	t.Fatalf("instance helper %q did not become ready", instanceID)
	return nil
}
