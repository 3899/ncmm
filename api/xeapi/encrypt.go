// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package xeapi

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const eapiKey = "e82ckenh8dichen8"

var xeapiStaticKey = mustHex("ab1d5a430f6bb04a3f01e81ddd72bd916d5ce591248ac128714806d7f8fb1b84")

func Encrypt(req EncryptRequest, publicKey PublicKeyState, session Session) (map[string]string, error) {
	if strings.TrimSpace(publicKey.PublicKey) == "" {
		return nil, ErrPublicKeyMissing
	}

	dynamicKey, activeSessionID, err := dynamicKey(session)
	if err != nil {
		return nil, err
	}

	plaintext, err := buildPlaintextEnvelope(req)
	if err != nil {
		return nil, err
	}
	inner, err := aesECBEncrypt(xeapiStaticKey, plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypt plaintext envelope: %w", err)
	}
	mid, err := midTransform(inner)
	if err != nil {
		return nil, err
	}
	b, err := aesECBEncrypt(dynamicKey, mid)
	if err != nil {
		return nil, fmt.Errorf("encrypt B: %w", err)
	}
	s, err := encryptS(dynamicKey, publicKey, firstNonEmpty(req.OS, defaultOS))
	if err != nil {
		return nil, err
	}
	r, err := aesECBEncrypt(xeapiStaticKey, []byte(publicKey.Version+"|"+activeSessionID))
	if err != nil {
		return nil, fmt.Errorf("encrypt R: %w", err)
	}

	return map[string]string{
		"B": base64.StdEncoding.EncodeToString(b),
		"S": base64.StdEncoding.EncodeToString(s),
		"R": base64.StdEncoding.EncodeToString(r),
	}, nil
}

func DecryptResponse(body []byte) ([]byte, error) {
	plaintext, err := aesECBDecrypt([]byte(eapiKey), body)
	if err != nil {
		return nil, fmt.Errorf("decrypt xeapi response: %w", err)
	}
	if len(plaintext) >= 2 && plaintext[0] == 0x1f && plaintext[1] == 0x8b {
		r, err := gzip.NewReader(bytes.NewReader(plaintext))
		if err != nil {
			return nil, fmt.Errorf("gzip.NewReader: %w", err)
		}
		defer r.Close()
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("gzip.ReadAll: %w", err)
		}
		return data, nil
	}
	return plaintext, nil
}

func DecryptPublicKey(encryptedData string) (PublicKeyState, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return PublicKeyState{}, fmt.Errorf("base64.DecodeString public key: %w", err)
	}
	plaintext, err := aesECBDecrypt(xeapiStaticKey, ciphertext)
	if err != nil {
		return PublicKeyState{}, fmt.Errorf("decrypt public key: %w", err)
	}
	var state PublicKeyState
	if err := json.Unmarshal(plaintext, &state); err != nil {
		return PublicKeyState{}, fmt.Errorf("json.Unmarshal public key: %w", err)
	}
	return state, nil
}

func dynamicKey(session Session) ([]byte, string, error) {
	if strings.TrimSpace(session.Key) != "" {
		key := []byte(session.Key)
		switch len(key) {
		case 16, 24, 32:
			return key, session.ID, nil
		default:
			return nil, "", fmt.Errorf("%w: got %d bytes", ErrSessionKeyLength, len(key))
		}
	}
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return nil, "", fmt.Errorf("rand.Read dynamic key: %w", err)
	}
	return key, "", nil
}

func midTransform(ciphertext []byte) ([]byte, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return nil, fmt.Errorf("rand.Read mid random: %w", err)
	}
	xored := make([]byte, len(ciphertext))
	for i := range ciphertext {
		xored[i] = ciphertext[i] ^ random[i&0x0f]
	}
	b64 := []byte(base64.StdEncoding.EncodeToString(xored))
	rot := 0
	if len(b64) > 0 {
		rot = int(random[0]&0x0f) % len(b64)
	}
	out := make([]byte, 0, len(random)+len(b64))
	out = append(out, random...)
	out = append(out, b64[rot:]...)
	out = append(out, b64[:rot]...)
	return out, nil
}

func encryptS(dynamicKey []byte, publicKey PublicKeyState, os string) ([]byte, error) {
	peerRaw, err := base64.StdEncoding.DecodeString(publicKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("base64.DecodeString peer public key: %w", err)
	}
	curve := ecdh.X25519()
	peer, err := curve.NewPublicKey(peerRaw)
	if err != nil {
		return nil, fmt.Errorf("x25519 peer public key: %w", err)
	}
	privateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("x25519 generate key: %w", err)
	}
	ephemeralRaw := privateKey.PublicKey().Bytes()
	sharedSecret, err := privateKey.ECDH(peer)
	if err != nil {
		return nil, fmt.Errorf("x25519 ECDH: %w", err)
	}
	aesKey := deriveX25519AESKey(sharedSecret, ephemeralRaw)
	iv := make([]byte, 12)
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("rand.Read gcm iv: %w", err)
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher S: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}
	plaintext := []byte(base64.StdEncoding.EncodeToString(dynamicKey) + "|" + os + "|" + publicKey.SK)
	encrypted := gcm.Seal(nil, iv, plaintext, nil)

	out := make([]byte, 0, len(ephemeralRaw)+len(iv)+len(encrypted))
	out = append(out, ephemeralRaw...)
	out = append(out, iv...)
	out = append(out, encrypted...)
	return out, nil
}

func deriveX25519AESKey(sharedSecret, ephemeralPublicKey []byte) []byte {
	if len(sharedSecret) == 0 {
		sharedSecret = make([]byte, 32)
	}
	prkMAC := hmac.New(sha256.New, make([]byte, 32))
	prkMAC.Write(sharedSecret)
	prk := prkMAC.Sum(nil)

	keyMAC := hmac.New(sha256.New, prk)
	keyMAC.Write(ephemeralPublicKey)
	keyMAC.Write([]byte{1})
	return keyMAC.Sum(nil)[:16]
}

func mustHex(value string) []byte {
	data, err := hex.DecodeString(value)
	if err != nil {
		panic(err)
	}
	return data
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func defaultMethod(method string) string {
	if strings.TrimSpace(method) == "" {
		return http.MethodPost
	}
	return method
}
