package atomicfile

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/3899/ncmm/internal/filelock"
)

func TestConcurrentReadersOnlyObserveCompleteFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	lockPath := path + ".lock"
	first := bytes.Repeat([]byte("a"), 128*1024)
	second := bytes.Repeat([]byte("b"), 128*1024)
	if err := Write(path, first, 0600); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 1)
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			lock, err := filelock.Acquire(context.Background(), lockPath)
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
			data, err := os.ReadFile(path)
			_ = lock.Close()
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
			if !bytes.Equal(data, first) && !bytes.Equal(data, second) {
				select {
				case errCh <- &partialReadError{size: len(data)}:
				default:
				}
				return
			}
		}
	}()
	for i := 0; i < 40; i++ {
		data := first
		if i%2 == 0 {
			data = second
		}
		lock, err := filelock.Acquire(context.Background(), lockPath)
		if err == nil {
			err = Write(path, data, 0600)
			_ = lock.Close()
		}
		if err != nil {
			close(done)
			wg.Wait()
			t.Fatal(err)
		}
	}
	close(done)
	wg.Wait()
	select {
	case err := <-errCh:
		t.Fatal(err)
	default:
	}
}

type partialReadError struct {
	size int
}

func TestAtomicWriteHelperProcess(t *testing.T) {
	if os.Getenv("NCMM_ATOMIC_WRITE_HELPER") != "1" {
		return
	}
	path := os.Getenv("NCMM_ATOMIC_WRITE_PATH")
	ready := os.Getenv("NCMM_ATOMIC_WRITE_READY")
	data := bytes.Repeat([]byte("b"), 512*1024)
	if err := os.WriteFile(ready, []byte("ready"), 0600); err != nil {
		os.Exit(2)
	}
	for {
		if err := Write(path, data, 0600); err != nil {
			os.Exit(3)
		}
	}
}

func TestKilledWriterLeavesCompleteDestination(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	ready := filepath.Join(dir, "ready")
	first := bytes.Repeat([]byte("a"), 512*1024)
	second := bytes.Repeat([]byte("b"), 512*1024)
	if err := Write(path, first, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestAtomicWriteHelperProcess$")
	cmd.Env = append(os.Environ(),
		"NCMM_ATOMIC_WRITE_HELPER=1",
		"NCMM_ATOMIC_WRITE_PATH="+path,
		"NCMM_ATOMIC_WRITE_READY="+ready,
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
		t.Fatal("atomic write helper did not become ready")
	}
	time.Sleep(25 * time.Millisecond)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, first) && !bytes.Equal(data, second) {
		t.Fatalf("destination contains partial data: %d bytes", len(data))
	}
}

func (e *partialReadError) Error() string {
	return "reader observed partial file of unexpected size"
}
