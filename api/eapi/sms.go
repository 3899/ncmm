// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package eapi

import (
	"context"

	"github.com/3899/ncmm/api/types"
)

type CaptchaSendReq struct {
	Phone  string
	CTCode string
}

type CaptchaSendResp struct {
	types.RespCommon[any]
}

// CaptchaSend 发送验证码 PC客户端
func (a *Api) CaptchaSend(ctx context.Context, req *CaptchaSendReq) (*CaptchaSendResp, error) {
	// TODO
	return nil, nil
}

type CaptchaVerifyReq struct {
	Phone   string `json:"phone"`
	CTCode  string `json:"ctcode"`
	Captcha string `json:"captcha"`
}

type CaptchaVerifyResp struct {
	types.RespCommon[any]
}

// CaptchaVerify 验证验证码
func (a *Api) CaptchaVerify(ctx context.Context, req *CaptchaVerifyReq) (*CaptchaVerifyResp, error) {
	// TODO
	return nil, nil
}
