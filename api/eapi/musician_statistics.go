// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package eapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/3899/ncmm/api"
	"github.com/3899/ncmm/api/types"
)

// MusicianSongDailyReq 获取音乐人昨日作品统计。
type MusicianSongDailyReq struct {
	MusicianEAPIReq
}

type MusicianSongDailyIndexes struct {
	Play int `json:"play"`
}

type MusicianSongDailyData struct {
	Indexes MusicianSongDailyIndexes `json:"indexs"`
}

type MusicianSongDailyResp struct {
	types.RespCommon[MusicianSongDailyData]
}

// MusicianSongDaily 获取官方音乐人中心展示的昨日有效播放量。
// 抓包接口: /api/nmusician/statistics/songs/daily
func (a *Api) MusicianSongDaily(ctx context.Context, req *MusicianSongDailyReq) (*MusicianSongDailyResp, error) {
	if req == nil {
		req = &MusicianSongDailyReq{}
	}
	a.fillMusicianEAPIReq(&req.MusicianEAPIReq)

	var (
		url   = "https://interface3.music.163.com/eapi/nmusician/statistics/songs/daily"
		reply MusicianSongDailyResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	if _, err := a.client.Request(ctx, url, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	return &reply, nil
}

// MusicianSongTrendReq 获取音乐人作品统计趋势。
type MusicianSongTrendReq struct {
	MusicianEAPIReq
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Type      int    `json:"type"`
}

type MusicianSongTrendPoint struct {
	DateTime string `json:"dateTime"`
	Value    string `json:"dataValue"`
}

type MusicianSongTrendResp struct {
	types.RespCommon[[]MusicianSongTrendPoint]
}

// MusicianSongTrend 获取指定日期范围内的官方每日有效播放量。
// type=100 表示播放量；接口通常不返回 EndTime 当日，由 songs/daily 补齐。
// 抓包接口: /api/creator/musician/song/summary/statistic/data/trend/get
func (a *Api) MusicianSongTrend(ctx context.Context, req *MusicianSongTrendReq) (*MusicianSongTrendResp, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if strings.TrimSpace(req.StartTime) == "" || strings.TrimSpace(req.EndTime) == "" {
		return nil, fmt.Errorf("startTime and endTime are required")
	}
	if req.Type == 0 {
		req.Type = 100
	}
	a.fillMusicianEAPIReq(&req.MusicianEAPIReq)

	var (
		url   = "https://interface3.music.163.com/eapi/creator/musician/song/summary/statistic/data/trend/get"
		reply MusicianSongTrendResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	if _, err := a.client.Request(ctx, url, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	return &reply, nil
}
