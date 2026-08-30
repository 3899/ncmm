package webui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/3899/ncmm/internal/filelock"
	"github.com/google/uuid"
)

const InstanceLockFilename = "webui.instance.lock"

var errWebUIAlreadyRunning = errors.New("WebUI instance is already running")

type instanceMetadata struct {
	PID        int       `json:"pid"`
	StartedAt  time.Time `json:"startedAt"`
	Listen     string    `json:"listen"`
	Version    string    `json:"version"`
	InstanceID string    `json:"instanceId"`
}

type InstanceInfo struct {
	PID        int       `json:"pid"`
	StartedAt  time.Time `json:"startedAt"`
	Listen     string    `json:"listen"`
	Version    string    `json:"version"`
	InstanceID string    `json:"instanceId"`
}

func InspectInstance(home string) (InstanceInfo, bool, error) {
	lock, err := acquireWebInstanceLock(home)
	if err == nil {
		if closeErr := lock.Close(); closeErr != nil {
			return InstanceInfo{}, false, closeErr
		}
		return InstanceInfo{}, false, nil
	}
	var running *instanceRunningError
	if !errors.As(err, &running) {
		return InstanceInfo{}, false, err
	}
	metadata := running.Metadata
	if running.ReadErr != nil {
		return InstanceInfo{}, true, running.ReadErr
	}
	return InstanceInfo{
		PID: metadata.PID, StartedAt: metadata.StartedAt, Listen: metadata.Listen,
		Version: metadata.Version, InstanceID: metadata.InstanceID,
	}, true, nil
}

type instanceRunningError struct {
	Home     string
	Metadata instanceMetadata
	ReadErr  error
}

func (e *instanceRunningError) Error() string {
	detail := "instance metadata is unavailable"
	if e.Metadata.PID > 0 || e.Metadata.Listen != "" || e.Metadata.InstanceID != "" {
		parts := make([]string, 0, 3)
		if e.Metadata.PID > 0 {
			parts = append(parts, fmt.Sprintf("pid=%d", e.Metadata.PID))
		}
		if e.Metadata.Listen != "" {
			parts = append(parts, "listen="+e.Metadata.Listen)
		}
		if e.Metadata.InstanceID != "" {
			parts = append(parts, "instanceId="+e.Metadata.InstanceID)
		}
		detail = strings.Join(parts, ", ")
	} else if e.ReadErr != nil {
		detail = "instance metadata is unavailable: " + e.ReadErr.Error()
	}
	return fmt.Sprintf("WebUI is already running for home %q (%s)", e.Home, detail)
}

func (e *instanceRunningError) Unwrap() error {
	return errWebUIAlreadyRunning
}

type webInstanceLock struct {
	home string
	path string
	lock *filelock.Lock
}

func acquireWebInstanceLock(home string) (*webInstanceLock, error) {
	normalized, err := normalizeHome(home)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(normalized, InstanceLockFilename)
	lock, err := filelock.TryAcquire(path)
	if err == nil {
		return &webInstanceLock{home: normalized, path: path, lock: lock}, nil
	}
	if !errors.Is(err, filelock.ErrLocked) {
		return nil, fmt.Errorf("acquire WebUI instance lock: %w", err)
	}
	metadata, readErr := readInstanceMetadata(path)
	return nil, &instanceRunningError{Home: normalized, Metadata: metadata, ReadErr: readErr}
}

func normalizeHome(home string) (string, error) {
	if strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("WebUI home is required")
	}
	abs, err := filepath.Abs(home)
	if err != nil {
		return "", fmt.Errorf("resolve WebUI home: %w", err)
	}
	abs = filepath.Clean(abs)
	if err := os.MkdirAll(abs, 0755); err != nil {
		return "", fmt.Errorf("create WebUI home: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("canonicalize WebUI home: %w", err)
	}
	return filepath.Clean(resolved), nil
}

func (l *webInstanceLock) writeMetadata(listen, version string) (instanceMetadata, error) {
	metadata := instanceMetadata{
		PID: os.Getpid(), StartedAt: time.Now().UTC(), Listen: listen,
		Version: version, InstanceID: uuid.NewString(),
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return instanceMetadata{}, err
	}
	data = append(data, '\n')
	if err := l.lock.Write(data); err != nil {
		return instanceMetadata{}, fmt.Errorf("write WebUI instance metadata: %w", err)
	}
	return metadata, nil
}

func (l *webInstanceLock) Close() error {
	if l == nil {
		return nil
	}
	return l.lock.Close()
}

func readInstanceMetadata(path string) (instanceMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return instanceMetadata{}, err
	}
	var metadata instanceMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return instanceMetadata{}, err
	}
	return metadata, nil
}
