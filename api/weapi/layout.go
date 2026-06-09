// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package weapi

import (
	"context"

	"github.com/3899/ncmm/api"
	"github.com/3899/ncmm/api/types"
)

// LayoutReq .
type LayoutReq struct {
	CsrfToken string `json:"csrf_token"`
}

type LayoutResp struct {
	types.RespCommon[any]
}

// Layout 退出
func (a *Api) Layout(ctx context.Context, req *LayoutReq) (*LayoutResp, error) {
	var (
		url  = "https://music.163.com/weapi/logout"
		resp LayoutResp
		opts = api.NewOptions()
	)
	if req.CsrfToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CsrfToken = csrf
	}
	reply, err := a.client.Request(ctx, url, req, &resp, opts)
	if err != nil {
		return nil, err
	}
	_ = reply
	return &resp, nil
}
