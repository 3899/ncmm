// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package eapi

import (
	"context"
	"fmt"

	"github.com/3899/ncmm/api"
	"github.com/3899/ncmm/api/types"
)

type YunBeiSignInReq struct {
	// Type 签到类型 0:安卓(默认)3点经验 1:web/PC2点经验
	Type int64 `json:"type"`
}

// YunBeiSignInResp 签到返回
type YunBeiSignInResp struct {
	// Code 错误码 -2:重复签到 200:成功(会有例外会出现“功能暂不支持”) 301:未登录
	types.RespCommon[any]
	// Point 签到获得积分奖励数量
	Point int64 `json:"point"`
}

// YunBeiSignIn 用户每日签到
// url:
// needLogin: 是
// todo:目前传0会出现功能暂不支持不知为何(可能请求头或cookie问题)待填坑
func (a *Api) YunBeiSignIn(ctx context.Context, req *YunBeiSignInReq) (*YunBeiSignInResp, error) {
	var (
		url   = "https://music.163.com/eapi/point/dailyTask"
		reply YunBeiSignInResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}
