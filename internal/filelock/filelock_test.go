package filelock

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestLockIsExclusiveAndReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml.lock")
	first, err := Acquire(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := Acquire(ctx, path); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second Acquire() error = %v; want deadline exceeded", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Acquire(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTryAcquireAndWriteMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.lock")
	first, err := TryAcquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Write([]byte("instance metadata\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := TryAcquire(path); !errors.Is(err, ErrLocked) {
		t.Fatalf("second TryAcquire() error = %v; want ErrLocked", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read metadata while locked: %v", err)
	}
	if string(data) != "instance metadata\n" {
		t.Fatalf("metadata = %q", data)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := TryAcquire(path)
	if err != nil {
		t.Fatalf("TryAcquire() after release: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLockHolderHelperProcess(t *testing.T) {
	if os.Getenv("NCMM_FILELOCK_HELPER") != "1" {
		return
	}
	lock, err := Acquire(context.Background(), os.Getenv("NCMM_FILELOCK_PATH"))
	if err != nil {
		os.Exit(2)
	}
	defer lock.Close()
	if err := os.WriteFile(os.Getenv("NCMM_FILELOCK_READY"), []byte("ready"), 0600); err != nil {
		os.Exit(3)
	}
	for {
		time.Sleep(time.Hour)
	}
}

func TestProcessExitReleasesLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml.lock")
	ready := filepath.Join(dir, "ready")
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHolderHelperProcess$")
	cmd.Env = append(os.Environ(),
		"NCMM_FILELOCK_HELPER=1",
		"NCMM_FILELOCK_PATH="+path,
		"NCMM_FILELOCK_READY="+ready,
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	readyObserved := false
	for time.Now().Before(deadline) {
		if _, err := os.Stat(ready); err == nil {
			readyObserved = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !readyObserved {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal("file lock helper did not become ready")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	if _, err := Acquire(ctx, path); !errors.Is(err, context.DeadlineExceeded) {
		cancel()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("Acquire() while child holds lock = %v; want deadline exceeded", err)
	}
	cancel()
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()

	lock, err := Acquire(context.Background(), path)
	if err != nil {
		t.Fatalf("Acquire() after child exit: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
}
