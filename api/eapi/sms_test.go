// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package eapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	phone = "your mobile phone number"
	ct    = "86"
)

func TestCaptchaSend(t *testing.T) {
	// 发送验证码
	var req = CaptchaSendReq{
		Phone:  phone,
		CTCode: ct,
	}
	got, err := cli.CaptchaSend(ctx, &req)
	assert.NoError(t, err)
	t.Logf("CaptchaSend: %+v\n", got)
}

func TestCaptchaVerify(t *testing.T) {
	// 发送验证码
	var req = CaptchaVerifyReq{
		Phone:   phone,
		CTCode:  ct,
		Captcha: "2129",
	}
	got, err := cli.CaptchaVerify(ctx, &req)
	assert.NoError(t, err)
	t.Logf("CaptchaVerify: %+v\n", got)
}
