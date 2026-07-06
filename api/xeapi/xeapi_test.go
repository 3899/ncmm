// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package xeapi

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSign(t *testing.T) {
	got := Sign("1779955010033", "4477405878624231")
	assert.Equal(t, "d6ouZ8bOiQrsH6kfslwG9zhJMvF6sJ4DCOlsGUkk7fw=", got)
}

func TestBuildPlaintextEnvelope(t *testing.T) {
	req := EncryptRequest{
		URI:    "/xeapi/demo/path?a=1",
		Method: http.MethodPost,
		Data: struct {
			Msg    string `json:"msg"`
			ER     bool   `json:"e_r"`
			Header string `json:"header,omitempty"`
		}{
			Msg:    "hello world",
			ER:     true,
			Header: "{}",
		},
	}
	plaintext, err := buildPlaintextEnvelope(req)
	require.NoError(t, err)

	var env plaintextEnvelope
	require.NoError(t, json.Unmarshal(plaintext, &env))
	assert.Equal(t, "a=1&e_r=true", env.QueryString)
	body, err := base64.StdEncoding.DecodeString(env.Body)
	require.NoError(t, err)
	assert.Equal(t, "header=%7B%7D&msg=hello+world", string(body))
}

func TestDecryptPublicKey(t *testing.T) {
	state := PublicKeyState{
		PublicKey:      "3m5wN9om11qRESjEV+5EoFf9qLEylO6gyThMbl1XxEk=",
		Version:        "1000000000000",
		NextUpdateTime: 1803882269000,
		SK:             "GYcibJw61779976227511",
	}
	data, err := json.Marshal(state)
	require.NoError(t, err)
	encrypted, err := aesECBEncrypt(xeapiStaticKey, data)
	require.NoError(t, err)

	got, err := DecryptPublicKey(base64.StdEncoding.EncodeToString(encrypted))
	require.NoError(t, err)
	assert.Equal(t, state, got)
}

func TestDecryptResponse(t *testing.T) {
	payload := []byte(`{"code":200,"data":true}`)
	encrypted, err := aesECBEncrypt([]byte(eapiKey), payload)
	require.NoError(t, err)

	got, err := DecryptResponse(encrypted)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

func TestDecryptGzipResponse(t *testing.T) {
	payload := []byte(`{"code":200,"data":{"ok":true}}`)
	var zipped bytes.Buffer
	w := gzip.NewWriter(&zipped)
	_, err := w.Write(payload)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	encrypted, err := aesECBEncrypt([]byte(eapiKey), zipped.Bytes())
	require.NoError(t, err)

	got, err := DecryptResponse(encrypted)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

func TestEncryptProducesBSR(t *testing.T) {
	got, err := Encrypt(
		EncryptRequest{
			URI:         "/xeapi/demo/path",
			Data:        map[string]interface{}{"header": "{}", "e_r": true},
			Method:      http.MethodPost,
			ContentType: "application/x-www-form-urlencoded;charset=utf-8",
			OS:          "android",
		},
		PublicKeyState{
			PublicKey: "3m5wN9om11qRESjEV+5EoFf9qLEylO6gyThMbl1XxEk=",
			Version:   "1000000000000",
			SK:        "GYcibJw61779976227511",
		},
		Session{ID: "ssid", Key: "1234567890123456"},
	)
	require.NoError(t, err)
	for _, key := range []string{"B", "S", "R"} {
		assert.NotEmpty(t, got[key])
		_, err := base64.StdEncoding.DecodeString(got[key])
		assert.NoError(t, err)
	}
}

func TestRawTimestampString(t *testing.T) {
	got, err := rawTimestampString(json.RawMessage(`1779955010033`))
	require.NoError(t, err)
	assert.Equal(t, "1779955010033", got)

	got, err = rawTimestampString(json.RawMessage(`"1779955010033"`))
	require.NoError(t, err)
	assert.Equal(t, "1779955010033", got)
}
