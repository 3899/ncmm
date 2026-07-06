// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package xeapi

import (
	"context"
	"strings"
	"sync"
	"time"
)

type ManagerOptions struct {
	HTTPClient HTTPDoer
	APIDomain  string
}

type Manager struct {
	mu       sync.Mutex
	provider publicKeyProvider
	key      PublicKeyState
	session  Session
}

func NewManager(opts ManagerOptions) *Manager {
	return &Manager{
		provider: &KeyClient{
			HTTPClient: opts.HTTPClient,
			APIDomain:  opts.APIDomain,
		},
	}
}

func (m *Manager) Encrypt(ctx context.Context, req EncryptRequest) (map[string]string, error) {
	if req.Method == "" {
		req.Method = defaultMethod(req.Method)
	}
	key, session, err := m.state(ctx, req)
	if err != nil {
		return nil, err
	}
	return Encrypt(req, key, session)
}

func (m *Manager) SetSession(id, key string) {
	id = strings.TrimSpace(id)
	key = strings.TrimSpace(key)
	if id == "" || key == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.session = Session{ID: id, Key: key}
}

func (m *Manager) state(ctx context.Context, req EncryptRequest) (PublicKeyState, Session, error) {
	m.mu.Lock()
	key := m.key
	session := m.session
	needsRefresh := key.PublicKey == "" || key.SK == "" || keyExpired(key)
	currentVersion := key.Version
	currentSK := key.SK
	m.mu.Unlock()

	if !needsRefresh {
		return key, session, nil
	}

	refreshed, err := m.provider.PublicKey(ctx, keyRefreshRequest{
		DeviceID:          req.DeviceID,
		CurrentKeyVersion: currentVersion,
		AppVersion:        req.AppVersion,
		UserAgent:         req.UserAgent,
		OS:                req.OS,
	})
	if err != nil {
		return PublicKeyState{}, Session{}, err
	}
	if refreshed.SK == "" {
		refreshed.SK = currentSK
	}
	if refreshed.SK == "" {
		return PublicKeyState{}, Session{}, ErrPublicKeyMissing
	}

	m.mu.Lock()
	m.key = refreshed
	session = m.session
	m.mu.Unlock()
	return refreshed, session, nil
}

func keyExpired(key PublicKeyState) bool {
	if key.NextUpdateTime <= 0 {
		return false
	}
	// nextUpdateTime is milliseconds in app responses.
	return time.Now().UnixMilli() >= key.NextUpdateTime
}
