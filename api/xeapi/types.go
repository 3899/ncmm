// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package xeapi

import (
	"context"
	"errors"
	"net/http"
)

const (
	defaultAPIDomain = "https://interface.music.163.com"
	defaultOS        = "android"
	defaultAppVer    = "9.1.65"
)

var (
	ErrPublicKeyMissing = errors.New("xeapi public key is missing")
	ErrSessionKeyLength = errors.New("xeapi session key length is invalid")
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type PublicKeyState struct {
	PublicKey      string `json:"publicKey"`
	Version        string `json:"version"`
	NextUpdateTime int64  `json:"nextUpdateTime"`
	SK             string `json:"sk"`
	DeviceID       string `json:"deviceId,omitempty"`
}

type Session struct {
	ID  string
	Key string
}

type EncryptRequest struct {
	URI         string
	Data        interface{}
	Method      string
	ContentType string
	OS          string
	AppVersion  string
	DeviceID    string
	UserAgent   string
}

type keyRefreshRequest struct {
	DeviceID          string
	CurrentKeyVersion string
	AppVersion        string
	UserAgent         string
	OS                string
}

type publicKeyProvider interface {
	PublicKey(ctx context.Context, req keyRefreshRequest) (PublicKeyState, error)
}
