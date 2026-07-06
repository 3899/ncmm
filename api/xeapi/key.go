// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package xeapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type KeyClient struct {
	HTTPClient HTTPDoer
	APIDomain  string
}

type keyResponse struct {
	Code int `json:"code"`
	Data struct {
		EncryptedData string          `json:"encryptedData"`
		Signature     string          `json:"signature"`
		Timestamp     json.RawMessage `json:"timestamp"`
	} `json:"data"`
	Message string `json:"message"`
}

func (c *KeyClient) PublicKey(ctx context.Context, req keyRefreshRequest) (PublicKeyState, error) {
	doer := c.HTTPClient
	if doer == nil {
		doer = http.DefaultClient
	}
	apiDomain := strings.TrimRight(c.APIDomain, "/")
	if apiDomain == "" {
		apiDomain = defaultAPIDomain
	}

	nonce, err := generateNonce()
	if err != nil {
		return PublicKeyState{}, err
	}
	timestamp := nowMillisString()
	appVersion := firstNonEmpty(req.AppVersion, defaultAppVer)
	osName := firstNonEmpty(req.OS, defaultOS)

	form := url.Values{}
	form.Set("appVersion", appVersion)
	form.Set("currentKeyVersion", req.CurrentKeyVersion)
	form.Set("deviceId", req.DeviceID)
	form.Set("nonce", nonce)
	form.Set("os", osName)
	form.Set("requestType", "active")
	form.Set("signature", Sign(timestamp, nonce))
	form.Set("t1", "")
	form.Set("t2", "")
	form.Set("timestamp", timestamp)
	form.Set("uid", "")

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiDomain+"/api/gorilla/anti/crawler/security/key/get",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return PublicKeyState{}, fmt.Errorf("http.NewRequest: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if ua := strings.TrimSpace(req.UserAgent); ua != "" {
		httpReq.Header.Set("User-Agent", ua)
	}
	if strings.TrimSpace(req.DeviceID) != "" {
		httpReq.Header.Set("Cookie", "deviceId="+url.QueryEscape(req.DeviceID))
	}

	resp, err := doer.Do(httpReq)
	if err != nil {
		return PublicKeyState{}, fmt.Errorf("public key request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return PublicKeyState{}, fmt.Errorf("read public key response: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return PublicKeyState{}, fmt.Errorf("public key http status %d: %s", resp.StatusCode, string(body))
	}

	var reply keyResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&reply); err != nil {
		return PublicKeyState{}, fmt.Errorf("json.Unmarshal public key response: %w", err)
	}
	if reply.Code != 200 || reply.Data.EncryptedData == "" {
		return PublicKeyState{}, fmt.Errorf("public key request failed: code=%d message=%s", reply.Code, reply.Message)
	}
	respTimestamp, err := rawTimestampString(reply.Data.Timestamp)
	if err != nil {
		return PublicKeyState{}, err
	}
	if reply.Data.Signature == "" || Sign(respTimestamp, nonce) != reply.Data.Signature {
		return PublicKeyState{}, fmt.Errorf("public key response signature mismatch")
	}

	state, err := DecryptPublicKey(reply.Data.EncryptedData)
	if err != nil {
		return PublicKeyState{}, err
	}
	state.DeviceID = req.DeviceID
	if strings.TrimSpace(state.SK) == "" {
		return PublicKeyState{}, fmt.Errorf("xeapi public key response missing sk")
	}
	return state, nil
}

func rawTimestampString(raw json.RawMessage) (string, error) {
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str, nil
	}
	var num json.Number
	if err := json.Unmarshal(raw, &num); err == nil {
		return num.String(), nil
	}
	return "", fmt.Errorf("public key response timestamp invalid")
}
