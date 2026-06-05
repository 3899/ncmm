// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package weapi

import (
	"context"
	"fmt"

	"github.com/3899/ncmm/api"
	"github.com/3899/ncmm/api/types"
)

type CDNListReq struct{}

type CDNListResp struct {
	types.RespCommon[[][]string]
}

// CDNList 获取CDN列表
// url: testdata/har/5.har
// needLogin: 未知
func (a *Api) CDNList(ctx context.Context, req *CDNListReq) (*CDNListResp, error) {
	var (
		url   = "https://music.163.com/weapi/cdns"
		reply CDNListResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}
