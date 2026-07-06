// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package xeapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/big"
	"strconv"
	"time"
)

const xeapiSignKey = "mUHCwVNWJbunMqAHf5MImuirT6plvs6VSFW62MGHstFQxhBGdEoIhLItH3djc4+FB/OKty3+lL2rGeoFBpVe5g=="

func Sign(timestamp, nonce string) string {
	mac := hmac.New(sha256.New, []byte(xeapiSignKey))
	mac.Write([]byte(timestamp))
	mac.Write([]byte(nonce))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func generateNonce() (string, error) {
	nonce := make([]byte, 16)
	for i := range nonce {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("rand.Int: %w", err)
		}
		nonce[i] = byte('0' + n.Int64())
	}
	return string(nonce), nil
}

func nowMillisString() string {
	return strconv.FormatInt(time.Now().UnixMilli(), 10)
}
