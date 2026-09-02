package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/3899/ncmm/internal/atomicfile"
	"github.com/3899/ncmm/internal/filelock"
)

type storeData struct {
	Version            int                `json:"version"`
	PasswordHash       string             `json:"passwordHash,omitempty"`
	ProtectionDisabled bool               `json:"protectionDisabled,omitempty"`
	Settings           Settings           `json:"settings"`
	Sessions           map[string]Session `json:"sessions"`
	UpdatedAt          time.Time          `json:"updatedAt"`
}

type store struct {
	path string
}

func newStore(path string) *store {
	return &store{path: filepath.Clean(path)}
}

func defaultStoreData() storeData {
	return storeData{Version: StoreVersion, Settings: DefaultSettings(), Sessions: make(map[string]Session)}
}

func (s *store) view(ctx context.Context, fn func(storeData) error) error {
	lock, err := filelock.Acquire(ctx, s.path+".lock")
	if err != nil {
		return err
	}
	defer lock.Close()
	data, err := s.read()
	if err != nil {
		return err
	}
	return fn(data)
}

func (s *store) update(ctx context.Context, fn func(*storeData) (bool, error)) error {
	lock, err := filelock.Acquire(ctx, s.path+".lock")
	if err != nil {
		return err
	}
	defer lock.Close()
	data, err := s.read()
	if err != nil {
		return err
	}
	dirty, err := fn(&data)
	if err != nil || !dirty {
		return err
	}
	data.Version = StoreVersion
	data.UpdatedAt = time.Now().UTC()
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return atomicfile.Write(s.path, encoded, 0600)
}

func (s *store) clear(ctx context.Context) error {
	lock, err := filelock.Acquire(ctx, s.path+".lock")
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *store) replace(ctx context.Context, data storeData) error {
	lock, err := filelock.Acquire(ctx, s.path+".lock")
	if err != nil {
		return err
	}
	defer lock.Close()
	data.Version = StoreVersion
	data.UpdatedAt = time.Now().UTC()
	if data.Sessions == nil {
		data.Sessions = make(map[string]Session)
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(s.path, append(encoded, '\n'), 0600)
}

func (s *store) read() (storeData, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return defaultStoreData(), nil
	}
	if err != nil {
		return storeData{}, fmt.Errorf("read authentication store: %w", err)
	}
	var decoded storeData
	if err := json.Unmarshal(data, &decoded); err != nil {
		return storeData{}, fmt.Errorf("parse authentication store: %w", err)
	}
	if decoded.Version != StoreVersion {
		return storeData{}, fmt.Errorf("unsupported authentication store version %d", decoded.Version)
	}
	if err := ValidateSettings(decoded.Settings); err != nil {
		return storeData{}, fmt.Errorf("invalid authentication settings: %w", err)
	}
	if decoded.Sessions == nil {
		decoded.Sessions = make(map[string]Session)
	}
	return decoded, nil
}
