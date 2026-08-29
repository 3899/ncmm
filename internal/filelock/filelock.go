package filelock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const retryInterval = 25 * time.Millisecond

var ErrLocked = errors.New("file lock is already held")

type Lock struct {
	file *os.File
}

func TryAcquire(path string) (*Lock, error) {
	file, err := open(path)
	if err != nil {
		return nil, err
	}
	locked, err := tryLock(file)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	if !locked {
		_ = file.Close()
		return nil, ErrLocked
	}
	return &Lock{file: file}, nil
}

func Acquire(ctx context.Context, path string) (*Lock, error) {
	file, err := open(path)
	if err != nil {
		return nil, err
	}
	for {
		locked, lockErr := tryLock(file)
		if lockErr != nil {
			_ = file.Close()
			return nil, fmt.Errorf("lock %s: %w", path, lockErr)
		}
		if locked {
			return &Lock{file: file}, nil
		}
		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = file.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func open(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (l *Lock) Write(data []byte) error {
	if l == nil || l.file == nil {
		return os.ErrInvalid
	}
	if err := l.file.Truncate(0); err != nil {
		return err
	}
	if _, err := l.file.Seek(0, 0); err != nil {
		return err
	}
	if _, err := l.file.Write(data); err != nil {
		return err
	}
	return l.file.Sync()
}

func (l *Lock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := unlock(l.file)
	closeErr := l.file.Close()
	l.file = nil
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
