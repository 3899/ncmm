// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

// Musician API
// Ported from https://github.com/NeteaseCloudMusicApiEnhanced/api-enhanced

package weapi

import (
	"context"
	"fmt"

	"github.com/3899/ncmm/api"
	"github.com/3899/ncmm/api/types"
)

// MusicianSignReq 音乐人签到请求
type MusicianSignReq struct{}

// MusicianSignResp 音乐人签到响应
type MusicianSignResp struct {
	types.RespCommon[any]
}

// MusicianSign 音乐人签到（完成"登录音乐人中心"任务）
// url: /weapi/creator/user/access
func (a *Api) MusicianSign(ctx context.Context, req *MusicianSignReq) (*MusicianSignResp, error) {
	var (
		url   = "https://music.163.com/weapi/creator/user/access"
		reply MusicianSignResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

// MusicianTasksReq 获取音乐人任务列表请求
type MusicianTasksReq struct{}

// MusicianTasksResp 获取音乐人任务列表响应
type MusicianTasksResp struct {
	types.RespCommon[MusicianTasksRespData]
}

// MusicianTasksRespData 音乐人任务列表数据
type MusicianTasksRespData struct {
	TaskList []MusicianTask `json:"taskList"`
}

// MusicianTask 单个音乐人任务
type MusicianTask struct {
	UserMissionId   int64  `json:"userMissionId"`
	MissionId       int64  `json:"missionId"`
	Period          int64  `json:"period"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Status          int64  `json:"status"` // 任务状态: 1=未完成, 2=已完成待领取, 3=已领取
	CurrentProgress int64  `json:"currentProgress"`
	TargetWorth     int64  `json:"targetWorth"`
	GrowthPoint     int64  `json:"growthPoint"`
	Action          string `json:"action"`
	ActionType      int64  `json:"actionType"`
	Type            int64  `json:"type"`
	UpdateTime      int64  `json:"updateTime"`
}

// MusicianTasks 获取音乐人周期任务列表
// url: /weapi/nmusician/workbench/mission/cycle/list
func (a *Api) MusicianTasks(ctx context.Context, req *MusicianTasksReq) (*MusicianTasksResp, error) {
	var (
		url   = "https://music.163.com/weapi/nmusician/workbench/mission/cycle/list"
		reply MusicianTasksResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

// MusicianTasksNewReq 获取音乐人阶段任务列表请求
type MusicianTasksNewReq struct{}

// MusicianTasksNewResp 获取音乐人阶段任务列表响应
type MusicianTasksNewResp struct {
	types.RespCommon[MusicianTasksRespData]
}

// MusicianTasksNew 获取音乐人阶段任务列表
// url: /weapi/nmusician/workbench/mission/stage/list
func (a *Api) MusicianTasksNew(ctx context.Context, req *MusicianTasksNewReq) (*MusicianTasksNewResp, error) {
	var (
		url   = "https://music.163.com/weapi/nmusician/workbench/mission/stage/list"
		reply MusicianTasksNewResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

// MusicianCloudbeanObtainReq 领取云豆请求
type MusicianCloudbeanObtainReq struct {
	UserMissionId string `json:"userMissionId"` // 任务 id (userMissionId)
	Period        string `json:"period"`        // 任务周期
}

// MusicianCloudbeanObtainResp 领取云豆响应
type MusicianCloudbeanObtainResp struct {
	types.RespCommon[any]
}

// MusicianCloudbeanObtain 领取音乐人云豆奖励
// url: /weapi/nmusician/workbench/mission/reward/obtain/new
func (a *Api) MusicianCloudbeanObtain(ctx context.Context, req *MusicianCloudbeanObtainReq) (*MusicianCloudbeanObtainResp, error) {
	if req.UserMissionId == "" {
		return nil, fmt.Errorf("userMissionId is required")
	}
	if req.Period == "" {
		return nil, fmt.Errorf("period is required")
	}

	var (
		url   = "https://music.163.com/weapi/nmusician/workbench/mission/reward/obtain/new"
		reply MusicianCloudbeanObtainResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

// MusicianVipInfoReq 音乐人 VIP 进阶权益与任务状态查询请求
type MusicianVipInfoReq struct{}

// MusicianVipInfoResp 音乐人 VIP 进阶权益与任务状态查询响应
type MusicianVipInfoResp struct {
	types.RespCommon[MusicianVipInfoData]
}

// MusicianVipInfoData 音乐人 VIP 状态数据 (对齐 weapi/nmusician/workbench/special/right/vip/info 响应)
type MusicianVipInfoData struct {
	HasOpen              bool                    `json:"hasOpen"`
	IsMusician           bool                    `json:"isMusician"`
	CanOpen              bool                    `json:"canOpen"`              // 当前是否可以开启/领取 VIP
	HasFurtherTask       bool                    `json:"hasFurtherTask"`
	TaskStatus           bool                    `json:"taskStatus"`           // 任务达标状态
	MusicianType         int                     `json:"musicianType"`
	Status               int                     `json:"status"`
	MaintainDays         int                     `json:"maintainDays"`
	RecentPlayCount30    int                     `json:"recentPlayCount30"`
	IsTodayStart         bool                    `json:"isTodayStart"`
	IsGrowthSupportUser  bool                    `json:"isGrowthSupportUser"`
	UnlockVipRight       bool                    `json:"unlockVipRight"`
	FurtherVipGetTime    int64                   `json:"furtherVipGetTime"`    // 可领取 VIP 的时间戳 (毫秒)
	FurtherTaskStartTime int64                   `json:"furtherTaskStartTime"`
	FurtherTask          *MusicianVipFurtherTask `json:"furtherTask"`
}

// MusicianVipFurtherTask 进阶任务
type MusicianVipFurtherTask struct {
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

// MusicianVipInfo 获取音乐人 VIP 进阶权益与任务状态 (WEAPI)
// url: /weapi/nmusician/workbench/special/right/vip/info
func (a *Api) MusicianVipInfo(ctx context.Context, req *MusicianVipInfoReq) (*MusicianVipInfoResp, error) {
	if req == nil {
		req = &MusicianVipInfoReq{}
	}
	var (
		url   = "https://interface.music.163.com/weapi/nmusician/workbench/special/right/vip/info"
		reply MusicianVipInfoResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeWEAPI
	opts.SetHeader("Referer", "https://y.music.163.com/")
	opts.SetHeader("Origin", "https://y.music.163.com")

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

// MusicianVipGetReq 领取音乐人黑胶 VIP 请求
type MusicianVipGetReq struct {
	CheckToken string `json:"checkToken,omitempty"`
}

// MusicianVipGetResp 领取音乐人黑胶 VIP 响应
type MusicianVipGetResp struct {
	types.RespCommon[bool]
}

// MusicianVipGet 领取音乐人黑胶 VIP (WEAPI)
// url: /weapi/nmusician/workbench/special/right/vip/get
func (a *Api) MusicianVipGet(ctx context.Context, req *MusicianVipGetReq, antiCheatToken string) (*MusicianVipGetResp, error) {
	if req == nil {
		req = &MusicianVipGetReq{}
	}
	if antiCheatToken != "" && req.CheckToken == "" {
		req.CheckToken = antiCheatToken
	}

	var (
		url   = "https://interface.music.163.com/weapi/nmusician/workbench/special/right/vip/get"
		reply MusicianVipGetResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeWEAPI
	opts.SetHeader("Referer", "https://y.music.163.com/")
	opts.SetHeader("Origin", "https://y.music.163.com")
	if antiCheatToken != "" {
		opts.SetHeader("x-anticheattoken", antiCheatToken)
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}
