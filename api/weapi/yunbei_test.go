// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package weapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestYunBeiSign(t *testing.T) {
	var req = YunBeiSignInReq{}
	got, err := cli.YunBeiSignIn(ctx, &req)
	assert.NoError(t, err)
	t.Logf("YunBeiSignIn: %+v\n", got)
}
