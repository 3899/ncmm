package webui

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const processSupervisorHelperMarker = "--process-supervisor-helper"

func TestProcessSupervisorHelperProcess(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == processSupervisorHelperMarker {
			index = i
			break
		}
	}
	if index < 0 || index+1 >= len(os.Args) {
		return
	}
	args := os.Args[index+1:]
	switch args[0] {
	case "output":
		_, _ = os.Stdout.WriteString(strings.Repeat("x", 4096))
		os.Exit(0)
	case "wait":
		_ = os.WriteFile(args[1], []byte("ready"), 0600)
		for {
			time.Sleep(time.Hour)
		}
	case "tree-parent":
		child := exec.Command(os.Args[0], "-test.run=^TestProcessSupervisorHelperProcess$", "--", processSupervisorHelperMarker, "tree-child", args[2])
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		_ = os.WriteFile(args[1], []byte("ready"), 0600)
		for {
			time.Sleep(time.Hour)
		}
	case "tree-child":
		time.Sleep(500 * time.Millisecond)
		_ = os.WriteFile(args[1], []byte("escaped"), 0600)
		os.Exit(0)
	}
	os.Exit(2)
}

func TestProcessSupervisorBoundsOutputAndRejectsAfterClose(t *testing.T) {
	supervisor := newProcessSupervisor()
	output, err := supervisor.RunOutput(context.Background(), processSpec{
		Kind: "output-test", Command: os.Args[0],
		Args: []string{"-test.run=^TestProcessSupervisorHelperProcess$", "--", processSupervisorHelperMarker, "output"},
	}, 128)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 128 || string(output) != strings.Repeat("x", 128) {
		t.Fatalf("bounded output length/content = %d, %q", len(output), output)
	}
	supervisor.Close(time.Second)
	if _, err := supervisor.Start(context.Background(), processSpec{Kind: "closed", Command: os.Args[0]}); err != errProcessSupervisorClosed {
		t.Fatalf("start after close error = %v; want %v", err, errProcessSupervisorClosed)
	}
}

func TestProcessSupervisorCloseWhileCommandIsBeingCreated(t *testing.T) {
	supervisor := newProcessSupervisor()
	creating := make(chan struct{})
	proceed := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		_, err := supervisor.Start(context.Background(), processSpec{
			Kind: "close-race", Command: os.Args[0],
			Args: []string{"-test.run=^TestProcessSupervisorHelperProcess$", "--", processSupervisorHelperMarker, "output"},
			CommandFactory: func(ctx context.Context, command string, args ...string) *exec.Cmd {
				close(creating)
				<-proceed
				return exec.CommandContext(ctx, command, args...)
			},
		})
		result <- err
	}()
	<-creating
	supervisor.Close(0)
	close(proceed)
	select {
	case err := <-result:
		if !errors.Is(err, errProcessSupervisorClosed) {
			t.Fatalf("Start() error = %v; want supervisor closed", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start() deadlocked while supervisor closed")
	}
}

func TestProcessSupervisorMarksExplicitStop(t *testing.T) {
	supervisor := newProcessSupervisor()
	defer supervisor.Close(time.Second)
	ready := filepath.Join(t.TempDir(), "ready")
	process, err := supervisor.Start(context.Background(), processSpec{
		Kind: "stop-test", Command: os.Args[0],
		Args: []string{"-test.run=^TestProcessSupervisorHelperProcess$", "--", processSupervisorHelperMarker, "wait", ready},
	})
	if err != nil {
		t.Fatal(err)
	}
	waitForFile(t, ready)
	process.Stop()
	if err := process.Wait(); err == nil {
		t.Fatal("stopped process returned no error")
	}
	if !process.StopRequested() {
		t.Fatal("explicit stop was not recorded")
	}
}

func TestProcessSupervisorReportsStartFailureAndTimeout(t *testing.T) {
	t.Run("start failure", func(t *testing.T) {
		supervisor := newProcessSupervisor()
		defer supervisor.Close(time.Second)
		if _, err := supervisor.Start(context.Background(), processSpec{Kind: "missing", Command: filepath.Join(t.TempDir(), "missing-command")}); err == nil || !strings.Contains(err.Error(), "start missing process") {
			t.Fatalf("start error = %v", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		supervisor := newProcessSupervisor()
		defer supervisor.Close(time.Second)
		ready := filepath.Join(t.TempDir(), "ready")
		process, err := supervisor.Start(context.Background(), processSpec{
			Kind: "timeout-test", Command: os.Args[0], Timeout: 100 * time.Millisecond,
			Args: []string{"-test.run=^TestProcessSupervisorHelperProcess$", "--", processSupervisorHelperMarker, "wait", ready},
		})
		if err != nil {
			t.Fatal(err)
		}
		waitForFile(t, ready)
		if err := process.Wait(); err == nil {
			t.Fatal("timed out process returned no error")
		}
		if !errors.Is(process.ContextError(), context.DeadlineExceeded) {
			t.Fatalf("context error = %v; want deadline exceeded", process.ContextError())
		}
		if process.StopRequested() {
			t.Fatal("timeout was incorrectly marked as an explicit stop")
		}
	})
}

func TestProcessSupervisorStopsDescendantProcess(t *testing.T) {
	supervisor := newProcessSupervisor()
	defer supervisor.Close(time.Second)
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	escaped := filepath.Join(dir, "escaped")
	process, err := supervisor.Start(context.Background(), processSpec{
		Kind: "tree-test", Command: os.Args[0],
		Args: []string{"-test.run=^TestProcessSupervisorHelperProcess$", "--", processSupervisorHelperMarker, "tree-parent", ready, escaped},
	})
	if err != nil {
		t.Fatal(err)
	}
	waitForFile(t, ready)
	process.Stop()
	_ = process.Wait()
	time.Sleep(800 * time.Millisecond)
	if _, err := os.Stat(escaped); !os.IsNotExist(err) {
		t.Fatalf("descendant process survived supervisor stop: %v", err)
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("helper file %q was not created", path)
}
