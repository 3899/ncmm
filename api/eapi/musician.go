// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

// Musician VIP Tasks API
// Ported from https://github.com/neteasecloudmusicapienhanced/api-enhanced
// Endpoint: /api/nmusician/workbench/special/right/vip/info (EAPI)

package eapi

import (
	"context"
	"fmt"

	"github.com/3899/ncmm/api"
	"github.com/3899/ncmm/api/types"
)

// MusicianVipTasksReq 获取音乐人黑胶会员任务请求
type MusicianVipTasksReq struct {
	ER bool `json:"e_r"` // false=明文响应, true=加密响应
}

// MusicianVipTasksData 获取音乐人黑胶会员任务响应 (data 字段内容)
type MusicianVipTasksData struct {
	HasOpen              bool                    `json:"hasOpen"`
	IsMusician           bool                    `json:"isMusician"`
	CanOpen              bool                    `json:"canOpen"`
	HasFurtherTask       bool                    `json:"hasFurtherTask"`
	TaskStatus           bool                    `json:"taskStatus"`
	MusicianType         int                     `json:"musicianType"`
	Status               int                     `json:"status"`
	MaintainDays         int                     `json:"maintainDays"`
	RecentPlayCount30    int                     `json:"recentPlayCount30"`
	IsTodayStart         bool                    `json:"isTodayStart"`
	IsGrowthSupportUser  bool                    `json:"isGrowthSupportUser"`
	UnlockVipRight       bool                    `json:"unlockVipRight"`
	FurtherVipGetTime    int64                   `json:"furtherVipGetTime"`
	FurtherTaskStartTime int64                   `json:"furtherTaskStartTime"`
	FurtherTask          *MusicianVipFurtherTask `json:"furtherTask"`
}

// MusicianVipTasksResp 获取音乐人黑胶会员任务响应
type MusicianVipTasksResp struct {
	types.RespCommon[MusicianVipTasksData]
}

// MusicianVipFurtherTask 进阶任务
type MusicianVipFurtherTask struct {
	Name             string                      `json:"name"`
	TotalCompleteNum int                         `json:"totalCompleteNum"`
	ProgressRate     int                         `json:"progressRate"`
	MissionStatus    int                         `json:"missionStatus"`
	MissionCode      string                      `json:"missionCode"`
	SortValue        int                         `json:"sortValue"`
	Desc             string                      `json:"desc"`
	TaskProgressText string                      `json:"taskProgressText"`
	Button           string                      `json:"button"`
	IconUrl          string                      `json:"iconUrl"`
	IosUrl           string                      `json:"iosUrl"`
	AndroidUrl       string                      `json:"androidUrl"`
	PcUrl            string                      `json:"pcUrl"`
	Children         []MusicianVipSubTask        `json:"children"`
}

// MusicianVipSubTask 子任务
type MusicianVipSubTask struct {
	Name             string               `json:"name"`
	TotalCompleteNum int                  `json:"totalCompleteNum"`
	ProgressRate     int                  `json:"progressRate"`
	MissionStatus    int                  `json:"missionStatus"`
	MissionCode      string               `json:"missionCode"`
	SortValue        int                  `json:"sortValue"`
	Desc             string               `json:"desc"`
	TaskProgressText string               `json:"taskProgressText"`
	Button           string               `json:"button"`
	IconUrl          string               `json:"iconUrl"`
	IosUrl           string               `json:"iosUrl"`
	AndroidUrl       string               `json:"androidUrl"`
	PcUrl            string               `json:"pcUrl"`
	Children         []MusicianVipSubTask `json:"children"`
}

// MusicianVipTasks 获取音乐人黑胶会员任务
// 接口: /api/nmusician/workbench/special/right/vip/info
// 加密: EAPI
// 需要登录
func (a *Api) MusicianVipTasks(ctx context.Context, req *MusicianVipTasksReq) (*MusicianVipTasksResp, error) {
	var (
		url   = "https://music.163.com/eapi/nmusician/workbench/special/right/vip/info"
		reply MusicianVipTasksResp
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
