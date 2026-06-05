// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package cookie

import (
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSync(t *testing.T) {
	filepath := os.TempDir() + "cookie.json"
	t.Cleanup(func() {
		_ = os.Remove(filepath)
	})

	jar, err := NewCookie(WithSyncInterval(0), WithFilePath(filepath))
	assert.NoError(t, err)

	u := &url.URL{Scheme: "https", Host: "example.com"}
	ck := []*http.Cookie{{Name: "token", Value: "pwd123"}, {Name: "email", Value: "test@example.com"}}
	jar.SetCookies(u, ck)

	data, err := os.ReadFile(filepath)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
	t.Logf("data:%s\n", string(data))
	// assert.JSONEq(t, string(data), target)
}
