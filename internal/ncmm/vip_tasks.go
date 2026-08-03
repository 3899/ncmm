// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package ncmm

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/3899/ncmm/api"
	"github.com/3899/ncmm/api/eapi"
	"github.com/3899/ncmm/api/types"
	"github.com/3899/ncmm/api/weapi"
)

type vipSignState struct {
	Today    bool
	Signed   bool
	RecordId int64
	Time     int64
	TimeStr  string
	Score    int64
}

type vipGrowthState struct {
	GrowthPoint         int64
	TodayScore          int64
	MonthTaskTotalScore int64
	CurrentDay          string
}

type vipMonthPrizeState struct {
	MonthCheckInTotalDay int64
	TodayDailyGrowth     int64
}

func newVipTaskListReq(deviceId string) *eapi.VipTaskListReq {
	return &eapi.VipTaskListReq{
		DeviceId: deviceId,
		OS:       "iOS",
		VerifyId: 1,
		Header:   struct{}{},
		IsNew:    1,
		ER:       true,
	}
}

func newVipSignInfoReq(deviceId string) *eapi.VipSignInfoReq {
	return &eapi.VipSignInfoReq{
		DeviceId: deviceId,
		OS:       "iOS",
		VerifyId: 1,
		Header:   struct{}{},
		ER:       true,
	}
}

func newVipGrowPointReq(deviceId string) *eapi.VipGrowPointReq {
	return &eapi.VipGrowPointReq{
		DeviceId: deviceId,
		OS:       "iOS",
		VerifyId: 1,
		Header:   struct{}{},
		ER:       true,
	}
}

func newVipCommonReq(deviceId string) *eapi.VipCommonReq {
	return &eapi.VipCommonReq{
		DeviceId: deviceId,
		OS:       "iOS",
		VerifyId: 1,
		Header:   struct{}{},
		ER:       true,
	}
}

func isVipSignTask(task eapi.VipTaskListData) bool {
	return task.MissionId == -500 ||
		strings.Contains(task.MainTitle, "乐签") ||
		(strings.Contains(task.MainTitle, "黑胶") && strings.Contains(task.MainTitle, "签"))
}

func isVipSignTaskDone(task eapi.VipTaskListData) bool {
	return task.Status == 100 ||
		strings.Contains(task.ButtonText, "已打卡") ||
		strings.Contains(task.ButtonText, "已完成")
}

func getVipSignState(ctx context.Context, request *eapi.Api, deviceId string) (vipSignState, error) {
	reply, err := request.VipSignInfo(ctx, newVipSignInfoReq(deviceId))
	if err != nil {
		return vipSignState{}, err
	}
	if reply.Code != 200 {
		return vipSignState{}, fmt.Errorf("code=%d msg=%s", reply.Code, reply.Message)
	}
	for _, info := range reply.Data {
		if !info.Today {
			continue
		}
		return vipSignState{
			Today:    true,
			Signed:   info.RecordId > 0 || info.Time > 0,
			RecordId: info.RecordId,
			Time:     info.Time,
			TimeStr:  info.TimeStr,
			Score:    info.Score,
		}, nil
	}
	return vipSignState{}, nil
}

func getVipGrowthState(ctx context.Context, request *eapi.Api, deviceId string) (vipGrowthState, error) {
	reply, err := request.VipGrowPoint(ctx, newVipGrowPointReq(deviceId))
	if err != nil {
		return vipGrowthState{}, err
	}
	if reply.Code != 200 {
		return vipGrowthState{}, fmt.Errorf("code=%d msg=%s", reply.Code, reply.Message)
	}

	state := vipGrowthState{GrowthPoint: reply.Data.UserLevel.GrowthPoint}
	if reply.Data.UserLevel.ExtJson == "" {
		return state, nil
	}

	var ext struct {
		TodayScore          int64  `json:"todayScore"`
		MonthTaskTotalScore int64  `json:"monthTaskTotalScore"`
		CurrentDay          string `json:"currentDay"`
	}
	if err := json.Unmarshal([]byte(reply.Data.UserLevel.ExtJson), &ext); err != nil {
		return state, nil
	}
	state.TodayScore = ext.TodayScore
	state.MonthTaskTotalScore = ext.MonthTaskTotalScore
	state.CurrentDay = ext.CurrentDay
	return state, nil
}

func getVipMonthPrizeState(ctx context.Context, request *eapi.Api, deviceId string) (vipMonthPrizeState, error) {
	reply, err := request.VipMonthPrizeList(ctx, newVipCommonReq(deviceId))
	if err != nil {
		return vipMonthPrizeState{}, err
	}
	if reply.Code != 200 {
		return vipMonthPrizeState{}, fmt.Errorf("code=%d msg=%s", reply.Code, reply.Message)
	}
	return vipMonthPrizeState{
		MonthCheckInTotalDay: reply.Data.MonthCheckInTotalDay,
		TodayDailyGrowth:     reply.Data.TodayDailyGrowth,
	}, nil
}

// getVipSignCompleted 综合判定乐签是否已完成。
// taskListData 为已获取的任务列表，传 nil 时内部自行获取（避免重复请求）。
// 优先级：月签进度（更准确） > sign/info 记录（兜底）。
func getVipSignCompleted(ctx context.Context, request *eapi.Api, deviceId string, taskListData []eapi.VipTaskListData) (bool, string) {
	// 1. 主判定：任务列表 + 月签进度
	var taskDone bool
	if taskListData != nil {
		for _, task := range taskListData {
			if !isVipSignTask(task) {
				continue
			}
			taskDone = isVipSignTaskDone(task)
			break
		}
	} else {
		taskList, taskErr := request.VipTaskList(ctx, newVipTaskListReq(deviceId))
		if taskErr == nil && taskList != nil {
			for _, task := range taskList.Data {
				if !isVipSignTask(task) {
					continue
				}
				taskDone = isVipSignTaskDone(task)
				break
			}
		}
	}
	if taskDone {
		month, monthErr := getVipMonthPrizeState(ctx, request, deviceId)
		if monthErr == nil && month.TodayDailyGrowth > 0 && month.MonthCheckInTotalDay > 0 {
			return true, fmt.Sprintf("任务中心已打卡，月签进度=%d天，今日成长值+%d", month.MonthCheckInTotalDay, month.TodayDailyGrowth)
		}
	}

	// 2. 兜底：sign/info 记录
	if state, err := getVipSignState(ctx, request, deviceId); err == nil && state.Signed {
		return true, fmt.Sprintf("乐签记录已确认，今日成长值+%d", state.Score)
	}

	return false, ""
}

func (c *SignIn) executeVipSign(ctx context.Context, request *eapi.Api, deviceId string) bool {
	c.cmd.Println("  👉 调用黑胶乐签接口...")
	resp, err := request.VipTaskSign(ctx, &eapi.VipTaskSignReq{
		Header: struct{}{},
		IsNew:  "1",
		ER:     true,
	})
	if err != nil {
		c.cmd.Printf("  ❌ 黑胶 VIP 乐签失败: %v\n", err)
		return false
	}
	if resp.Code == 200 {
		c.cmd.Println("  ✅ 黑胶 VIP 乐签成功")
		return true
	}
	c.cmd.Printf("  ⚠️ 黑胶 VIP 乐签异常: code=%d msg=%s\n", resp.Code, resp.Message)
	return false
}

func (c *SignIn) executeSingleVipTask(ctx context.Context, cli *api.Client, request *weapi.Api, eapiRequest *eapi.Api, v eapi.VipTaskListData, deviceId string, userLevel int64, likedSongIds *[]string) {
	if v.MissionCode == "HXSSG" || strings.Contains(v.MainTitle, "红心3首") {
		c.cmd.Println("  👉 开始执行 [红心3首VIP单曲]...")
		ids := c.doVipSongLike(ctx, request, eapiRequest)
		if len(ids) > 0 && likedSongIds != nil {
			*likedSongIds = append(*likedSongIds, ids...)
		}
	}

	if v.MissionCode == "MRGSSVIPGQ" || strings.Contains(v.MainTitle, "听3首VIP") || strings.Contains(v.MainTitle, "听3首会员") {
		c.cmd.Println("  👉 开始执行 [每日听3首VIP歌曲]...")
		c.doVipSongListen(ctx, cli, request, eapiRequest, deviceId)
	}

	if strings.Contains(v.MainTitle, "调音") {
		c.cmd.Println("  👉 开始执行 [查看AI调音大师]...")
		c.doVipSimulateBrowse(ctx, cli, request, v.JumpUrl, 16, 20, "查看AI调音大师")
	}

	if strings.Contains(v.MainTitle, "云贝") {
		c.cmd.Println("  👉 开始执行 [浏览云贝中心]...")
		c.doVipSimulateBrowse(ctx, cli, request, v.JumpUrl, 16, 20, "浏览云贝中心")
	}

	if strings.Contains(v.MainTitle, "分享") {
		c.cmd.Println("  👉 开始执行 [分享单曲到站外]...")
		c.doVipSimulateBrowse(ctx, cli, request, v.JumpUrl, 3, 5, "分享单曲到站外")
		c.doVipSongShare(ctx, request, eapiRequest)
	}

	if v.MissionCode == "FLQ" || strings.Contains(v.MainTitle, "领福利") {
		c.cmd.Println("  👉 开始执行 [免费领福利]...")
		taskIdStr := strconv.FormatInt(v.MissionId, 10)
		if taskIdStr != "" && taskIdStr != "0" {
			jumpUrl := v.JumpUrl
			if !strings.Contains(jumpUrl, "view_task_id=") {
				if strings.Contains(jumpUrl, "?") {
					jumpUrl = fmt.Sprintf("%s&view_task_id=%s&view_task_business=music.vip_growth&view_time=15", jumpUrl, taskIdStr)
				} else {
					jumpUrl = fmt.Sprintf("%s?view_task_id=%s&view_task_business=music.vip_growth&view_time=15", jumpUrl, taskIdStr)
				}
			}
			c.doVipSimulateBrowse(ctx, cli, request, jumpUrl, 16, 20, "免费领福利")
		}
		c.doVipWelfareClaim(ctx, request, eapiRequest, userLevel)
	}
}

func (c *SignIn) handleVipTasks(ctx context.Context, cli *api.Client, request *weapi.Api, userLevel int64) {
	enableVipTask := true
	if c.root.Cfg.Sign != nil && c.root.Cfg.Sign.EnableVipTask != nil {
		enableVipTask = *c.root.Cfg.Sign.EnableVipTask
	}

	if !enableVipTask {
		// 原有的极简 WEAPI 黑胶签到与领取逻辑
		vipSign, err := request.VipTaskSign(ctx, &weapi.VipTaskSignReq{IsNew: ""})
		if err == nil && vipSign.Data {
			c.cmd.Println("  ✅ 黑胶 VIP 乐签成功")
		} else {
			c.cmd.Println("  黑胶 VIP 今天已乐签过")
		}

		if c.isAutomatic() {
			reward, err := request.VipRewardGetAll(ctx, &weapi.VipRewardGetAllReq{})
			if err == nil && reward.Data.Result {
				c.cmd.Println("  ✅ 黑胶成长值已一键领取成功")
			}
		}
		return
	}

	eapiRequest := eapi.New(cli)
	var likedSongIds []string

	deviceId := cli.GetDeviceId()

	if deviceId != "" {
		c.cmd.Printf("  👉 [deviceId] 使用本地已缓存的唯一设备 ID: %s\n", deviceId)
	} else {
		c.cmd.Println("  👉 [deviceId] 缓存中无有效设备 ID，本次打卡将不传递设备 ID 字段以规避验证风险。")
	}

	// 0. 获取今日乐签状态作为真实现状。
	var signedToday bool
	beforeTodayScore := int64(-1)
	if growth, err := getVipGrowthState(ctx, eapiRequest, deviceId); err == nil {
		beforeTodayScore = growth.TodayScore
		c.cmd.Printf("  👉 [执行前] 黑胶成长值现状: 今日已获得 %d，当前成长值 %d\n", growth.TodayScore, growth.GrowthPoint)
	} else {
		c.cmd.Printf("  ⚠️ 获取黑胶成长值现状失败: %v\n", err)
	}

	for round := 1; round <= 3; round++ {
		c.cmd.Printf("  👉 [黑胶任务第 %d/3 轮] 获取黑胶 VIP 任务列表...\n", round)
		taskList, err := eapiRequest.VipTaskList(ctx, newVipTaskListReq(deviceId))
		if err != nil || taskList.Code != 200 {
			c.cmd.Printf("  ❌ 获取黑胶 VIP 任务列表失败: %v\n", err)
			return
		}

		allTasks := taskList.Data
		if progressList, err := getVipProgressTasks(ctx, request); err == nil {
			existing := make(map[string]bool)
			for _, t := range allTasks {
				if t.MissionCode != "" {
					existing[t.MissionCode] = true
				}
				existing[t.MainTitle] = true
			}
			for _, t := range progressList {
				isDup := false
				if t.MissionCode != "" && existing[t.MissionCode] {
					isDup = true
				}
				if existing[t.MainTitle] {
					isDup = true
				}
				if !isDup {
					allTasks = append(allTasks, t)
				}
			}
		} else {
			c.cmd.Printf("  ⚠️ 获取成长中心任务列表失败: %v\n", err)
		}

		var signVerifyReason string
		if !signedToday {
			if ok, reason := getVipSignCompleted(ctx, eapiRequest, deviceId, allTasks); ok {
				signedToday = true
				signVerifyReason = reason
			}
		}

		// 筛选待做任务
		var todoTasks []eapi.VipTaskListData
		var hasVipSignTask bool
		for _, v := range allTasks {
			if isVipSignTask(v) {
				hasVipSignTask = true
				if !signedToday {
					todoTasks = append(todoTasks, v)
				}
				continue
			}
			if v.Status != 100 {
				todoTasks = append(todoTasks, v)
			}
		}

		// 若无待做任务且已经签到（或者压根没有签到任务），提前终止循环
		if len(todoTasks) == 0 && (signedToday || !hasVipSignTask) {
			c.cmd.Printf("  ℹ️ [黑胶任务第 %d/3 轮] 无待执行的未完成任务，退出循环\n", round)
			break
		}

		c.cmd.Printf("  👉 [黑胶任务第 %d/3 轮] 待完成任务列表:\n", round)
		for _, v := range todoTasks {
			worth := v.Worth
			if worth == 0 && isVipSignTask(v) {
				worth = 3
			}
			c.cmd.Printf("    - 任务: %-15s | 奖励: %d成长值\n", v.MainTitle, worth)
		}
		if signVerifyReason != "" {
			c.cmd.Printf("  ✅ 乐签状态验证通过: %s\n", signVerifyReason)
		}

		// 执行待做任务
		for i, v := range todoTasks {
			if isVipSignTask(v) {
				c.cmd.Println("  👉 开始执行 [黑胶乐签打卡]...")
				signedToday = c.executeVipSign(ctx, eapiRequest, deviceId)
				if signedToday && beforeTodayScore >= 0 {
					if growth, err := getVipGrowthState(ctx, eapiRequest, deviceId); err == nil && growth.TodayScore != beforeTodayScore {
						c.cmd.Printf("  👉 黑胶今日成长值变化: %d -> %d\n", beforeTodayScore, growth.TodayScore)
					}
				}
				if !signedToday {
					c.cmd.Println("  ❌ 签到验证失败: 未找到今日有效的签到记录 (今日可能仍未成功打卡)")
				}
			} else {
				// 其他常规任务
				c.executeSingleVipTask(ctx, cli, request, eapiRequest, v, deviceId, userLevel, &likedSongIds)
			}
			if i < len(todoTasks)-1 {
				c.sleepBetweenSubtasks(ctx, v.MainTitle)
			}
		}

		// 兜底签到逻辑
		if !hasVipSignTask && !signedToday {
			c.cmd.Println("  ⚠️ 任务列表未返回黑胶乐签条目，但 sign/info 显示今日未落库，直接执行黑胶乐签打卡")
			signedToday = c.executeVipSign(ctx, eapiRequest, deviceId)
			if !signedToday {
				c.cmd.Println("  ❌ 签到验证失败: 未找到今日有效的签到记录 (今日可能仍未成功打卡)")
			}
		}

		// 一键领取所有已完成的成长值
		if c.isAutomatic() {
			c.cmd.Println("  👉 正在一键领取所有黑胶 VIP 成长值...")
			eapiReward, err := eapiRequest.VipRewardGetAll(ctx, &eapi.VipRewardGetAllReq{
				DeviceId: deviceId,
				OS:       "iOS",
				VerifyId: 1,
				Header:   struct{}{},
				ER:       true,
			})
			if err == nil && eapiReward.Code == 200 && eapiReward.Data.Result {
				c.cmd.Println("  ✅ 成功领取所有黑胶 VIP 成长值 (EAPI)")
			} else {
				// EAPI 失败时，回退到 weapi 版
				weapiReward, err := request.VipRewardGetAll(ctx, &weapi.VipRewardGetAllReq{})
				if err == nil && weapiReward.Data.Result {
					c.cmd.Println("  ✅ 成功领取所有黑胶 VIP 成长值 (WEAPI)")
				} else {
					c.cmd.Printf("  ⚠️ 一键领取成长值结果不明确，可能有部分已领取或需手动核对。EAPI: %v, WEAPI: %v\n", err, err)
				}
			}
		}
	}

	// 最终状态拉取与比对展示
	finalList, err := eapiRequest.VipTaskList(ctx, newVipTaskListReq(deviceId))
	if err == nil && finalList.Code == 200 {
		c.cmd.Println("  👉 最终的黑胶 VIP 任务列表状态:")
		allFinalTasks := finalList.Data
		if progressList, err := getVipProgressTasks(ctx, request); err == nil {
			existing := make(map[string]bool)
			for _, t := range allFinalTasks {
				if t.MissionCode != "" {
					existing[t.MissionCode] = true
				}
				existing[t.MainTitle] = true
			}
			for _, t := range progressList {
				isDup := false
				if t.MissionCode != "" && existing[t.MissionCode] {
					isDup = true
				}
				if existing[t.MainTitle] {
					isDup = true
				}
				if !isDup {
					allFinalTasks = append(allFinalTasks, t)
				}
			}
		}
		for _, v := range allFinalTasks {
			statusStr := "未完成"
			if v.Status == 100 {
				statusStr = "已完成"
			}
			if isVipSignTask(v) {
				if signedToday {
					statusStr = "已完成"
				} else {
					statusStr = "未完成"
				}
			}
			worth := v.Worth
			if worth == 0 && isVipSignTask(v) {
				worth = 3
			}
			c.cmd.Printf("    - 任务: %-15s | 状态: %-6s | 奖励: %d成长值\n", v.MainTitle, statusStr, worth)
		}
	} else {
		c.cmd.Printf("  ❌ 无法获取最终对照任务列表: %v\n", err)
	}

	// 再次获取并展示今日乐签和成长值状态以作对比
	c.cmd.Println("  👉 获取执行后黑胶 VIP 状态以作对比...")
	if finalGrowth, err := getVipGrowthState(ctx, eapiRequest, deviceId); err == nil {
		c.cmd.Printf("  👉 [执行后] 黑胶成长值最终状态: 今日已获得 %d，当前成长值 %d\n", finalGrowth.TodayScore, finalGrowth.GrowthPoint)
	} else {
		c.cmd.Printf("  ⚠️ 获取黑胶成长值最终状态失败: %v\n", err)
	}
	if finalSignState, err := getVipSignState(ctx, eapiRequest, deviceId); err == nil {
		if finalSignState.Today {
			c.cmd.Printf("  👉 [执行后] 乐签最终记录: recordId=%d time=%d score=%d\n", finalSignState.RecordId, finalSignState.Time, finalSignState.Score)
		}
	} else {
		c.cmd.Printf("  ⚠️ 获取黑胶乐签最终记录失败: %v\n", err)
	}

	// 统一取消红心，避免干扰用户歌单
	if len(likedSongIds) > 0 {
		c.cmd.Printf("  👉 统一取消 %d 首热门 VIP 歌曲的红心，彻底恢复用户歌单...\n", len(likedSongIds))
		for _, songIdStr := range likedSongIds {
			_, _ = eapiRequest.SongLike(ctx, &eapi.SongLikeReq{
				TrackId:    songIdStr,
				Like:       "false",
				Time:       "3",
				CheckToken: "",
			})
			sleepSec := 1 + rand.Intn(3)
			time.Sleep(time.Duration(sleepSec) * time.Second)
		}
		c.cmd.Println("  ✅ 统一取消红心完成")
	}
}

// doVipSongLike 专门操作热门 VIP 歌曲，不干扰个人收藏歌单
func (c *SignIn) doVipSongLike(ctx context.Context, request *weapi.Api, eapiRequest *eapi.Api) []string {
	// 获取热门 VIP 歌曲歌单 8402996200
	detail, err := request.PlaylistDetail(ctx, &weapi.PlaylistDetailReq{
		Id: "8402996200",
		N:  "10",
		S:  "0",
	})
	if err != nil || detail.Code != 200 || len(detail.Playlist.TrackIds) == 0 {
		c.cmd.Printf("  ❌ 获取热门 VIP 歌曲歌单失败，取消操作以避嫌: %v\n", err)
		return nil
	}

	trackIds := detail.Playlist.TrackIds
	c.cmd.Printf("  👉 成功获取热门 VIP 歌单，包含 %d 首歌曲，准备随机挑选 3 首进行红心打卡...\n", len(trackIds))

	n := len(trackIds)
	count := 3
	if n < 3 {
		count = n
	}

	// 随机打乱下标选择 3 首歌
	indices := rand.Perm(n)
	var likedIds []string
	successCount := 0
	for i := 0; i < count; i++ {
		idx := indices[i]
		songId := trackIds[idx].Id
		songIdStr := fmt.Sprintf("%d", songId)

		// 1. 红心该热门歌曲
		_, likeErr := eapiRequest.SongLike(ctx, &eapi.SongLikeReq{
			TrackId:    songIdStr,
			Like:       "true",
			Time:       "3",
			CheckToken: "",
		})
		if likeErr != nil {
			c.cmd.Printf("    ❌ [%d/3] 红心歌曲 ID %d 失败: %v\n", i+1, songId, likeErr)
			continue
		}

		// 2. 模拟播放器停留避嫌
		sleepSec := 3 + rand.Intn(8) // 3 ~ 10 秒
		c.cmd.Printf("    ⏳ [%d/3] 歌曲 ID %d 已红心，模拟在播放器停留 %d 秒以避嫌...\n", i+1, songId, sleepSec)

		select {
		case <-ctx.Done():
			c.cmd.Println("    ❌ 任务被取消")
			return likedIds
		case <-time.After(time.Duration(sleepSec) * time.Second):
		}

		likedIds = append(likedIds, songIdStr)
		successCount++
	}
	c.cmd.Printf("  ✅ 红心 3 首 VIP 单曲操作完毕，成功: %d/%d (暂保留红心以待领取奖励)\n", successCount, count)
	return likedIds
}

// doVipSongListen 听VIP歌曲任务打卡
func (c *SignIn) doVipSongListen(ctx context.Context, cli *api.Client, request *weapi.Api, eapiRequest *eapi.Api, deviceId string) {
	// 获取热门 VIP 歌曲歌单 8402996200
	detail, err := request.PlaylistDetail(ctx, &weapi.PlaylistDetailReq{
		Id: "8402996200",
		N:  "10",
		S:  "0",
	})
	if err != nil || detail.Code != 200 || len(detail.Playlist.TrackIds) == 0 {
		c.cmd.Printf("  ❌ 获取热门 VIP 歌曲歌单失败: %v\n", err)
		return
	}

	trackIds := detail.Playlist.TrackIds
	n := len(trackIds)
	count := 3
	if n < 3 {
		count = n
	}

	// 随机打乱选择 3 首歌
	indices := rand.Perm(n)
	selectedSongIds := make([]string, count)
	for i := 0; i < count; i++ {
		selectedSongIds[i] = fmt.Sprintf("%d", trackIds[indices[i]].Id)
	}

	// 批量查询歌曲详情以获得正确的 AlbumId、歌名和歌曲时长 (dt)
	reqList := make([]weapi.SongDetailReqList, len(selectedSongIds))
	for i, id := range selectedSongIds {
		reqList[i] = weapi.SongDetailReqList{Id: id, V: 0}
	}
	albumMap := make(map[int64]int64)
	nameMap := make(map[int64]string)
	durationMap := make(map[int64]int64)

	details, detailErr := request.SongDetail(ctx, &weapi.SongDetailReq{C: reqList})
	if detailErr == nil && details != nil {
		for _, s := range details.Songs {
			albumMap[s.Id] = s.Al.Id
			nameMap[s.Id] = s.Name
			durationMap[s.Id] = s.Dt
		}
	}

	c.cmd.Printf("  👉 成功获取热门 VIP 歌单，包含 %d 首歌曲，准备随机挑选 %d 首进行听歌打卡...\n", n, count)

	successCount := 0
	for i := 0; i < count; i++ {
		songId := trackIds[indices[i]].Id
		songIdStr := fmt.Sprintf("%d", songId)

		albumId := albumMap[songId]
		albumIdStr := fmt.Sprintf("%d", albumId)

		dt := durationMap[songId]
		durationSec := int(dt / 1000)
		if durationSec <= 0 {
			durationSec = 180 // 兜底默认 3 分钟
		}

		// 1. 模拟播放器拉流 (SongPlayerV1 / trialMode 31) 并发起真实音频流 CDN 请求
		startTimeSec := time.Now().Unix()
		playerResp, playerErr := request.SongPlayerV1(ctx, &weapi.SongPlayerV1Req{
			Ids:        types.IntsString{songId},
			Level:      types.LevelStandard,
			EncodeType: "aac",
			TrialMode:  "31",
		})
		if playerErr == nil && playerResp != nil && playerResp.Code == 200 && len(playerResp.Data) > 0 {
			if songUrl := playerResp.Data[0].Url; songUrl != "" {
				_ = downloadAudioToBuffer(ctx, cli, songUrl)
			}
		}

		// 2. 按歌曲完整时长模拟播放停留与动态进度条
		songName := nameMap[songId]
		var label string
		if songName != "" {
			label = fmt.Sprintf("[%d/%d] 正在听 VIP 歌曲《%s》(ID: %d)", i+1, count, songName, songId)
		} else {
			label = fmt.Sprintf("[%d/%d] 正在听 VIP 歌曲 (ID: %d)", i+1, count, songId)
		}
		c.sleepWithProgress(ctx, label, time.Duration(durationSec)*time.Second)

		// 3. 上报 VIP 听歌时长与活动充值卡记录 (VipActivityRefillCardListenInfo)
		refillReq := &eapi.VipActivityRefillCardListenInfoReq{
			BizExtend:  fmt.Sprintf("{\"songId\":\"%s\"}", songIdStr),
			ResourceId: fmt.Sprintf("%s_%d", songIdStr, startTimeSec),
			IncrAmount: durationSec,
		}
		_, _ = eapiRequest.VipActivityRefillCardListenInfo(ctx, refillReq)

		// 4. 上报听歌打卡 (TrialsongListen, Scene: 31, 真实 AlbumId)
		listenReq := &eapi.TrialsongListenReq{
			SongId:  songIdStr,
			AlbumId: albumIdStr,
			Scene:   31,
		}

		resp, listenErr := eapiRequest.TrialsongListen(ctx, listenReq)
		if listenErr != nil {
			c.cmd.Printf("    ❌ [%d/%d] 听歌上报 ID %d 失败: %v\n", i+1, count, songId, listenErr)
		} else if resp.Code != 200 {
			c.cmd.Printf("    ⚠️ [%d/%d] 听歌上报 ID %d 提示异常: code=%d msg=%s\n", i+1, count, songId, resp.Code, resp.Message)
		} else {
			c.cmd.Printf("    ✅ [%d/%d] 听歌上报 ID %d 成功\n", i+1, count, songId)
			successCount++
		}

		// 4. 切歌过渡等待 2~5 秒
		if i < count-1 {
			transitionSleep := 2 + rand.Intn(4)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(transitionSleep) * time.Second):
			}
		}
	}
	c.cmd.Printf("  ✅ 听 3 首 VIP 歌曲操作完毕，成功: %d/%d\n", successCount, count)
}

// doVipSongShare 专门分享热门 VIP 歌曲，用于打卡“分享单曲到站外”任务
func (c *SignIn) doVipSongShare(ctx context.Context, request *weapi.Api, eapiRequest *eapi.Api) {
	// 获取热门 VIP 歌曲歌单 8402996200
	detail, err := request.PlaylistDetail(ctx, &weapi.PlaylistDetailReq{
		Id: "8402996200",
		N:  "10",
		S:  "0",
	})
	if err != nil || detail.Code != 200 || len(detail.Playlist.TrackIds) == 0 {
		c.cmd.Printf("  ❌ 获取热门 VIP 歌曲歌单失败，无法执行分享: %v\n", err)
		return
	}

	trackIds := detail.Playlist.TrackIds
	n := len(trackIds)
	if n == 0 {
		c.cmd.Println("  ❌ 热门 VIP 歌单歌曲为空")
		return
	}

	// 随机挑选一首进行分享
	idx := rand.Intn(n)
	songId := trackIds[idx].Id
	songIdStr := fmt.Sprintf("%d", songId)

	c.cmd.Printf("  👉 随机选择 VIP 歌曲 ID %d 进行站外分享...\n", songId)

	// 调用 DailySongShareTrigger 进行分享（优先 wxsession 微信会话，失败时尝试 copylink 复制链接兜底）
	reqObj := &eapi.DailySongShareTriggerReq{
		SongID:  songIdStr,
		Channel: "wxsession",
	}
	reqObj.CryptoMode = api.CryptoModeEAPI
	resp, shareErr := eapiRequest.DailySongShareTrigger(ctx, reqObj)

	if shareErr != nil || resp.Code != 200 || !resp.Data {
		if shareErr != nil {
			c.cmd.Printf("  ⚠️ 微信分享(wxsession)上报失败: %v，尝试复制链接(copylink)兜底...\n", shareErr)
		} else {
			c.cmd.Printf("  ⚠️ 微信分享(wxsession)上报异常: code=%d msg=%s data=%v，尝试复制链接(copylink)兜底...\n", resp.Code, resp.Message, resp.Data)
		}
		// 复制链接 (copylink) 兜底
		reqObj.Channel = "copylink"
		resp, shareErr = eapiRequest.DailySongShareTrigger(ctx, reqObj)
		if shareErr != nil {
			c.cmd.Printf("  ❌ 复制链接(copylink)分享上报失败: %v\n", shareErr)
		} else if resp.Code != 200 || !resp.Data {
			c.cmd.Printf("  ⚠️ 复制链接(copylink)分享上报异常: code=%d msg=%s data=%v\n", resp.Code, resp.Message, resp.Data)
		} else {
			c.cmd.Printf("  ✅ 复制链接(copylink)分享单曲上报成功 (SongID: %s)\n", songIdStr)
		}
	} else {
		c.cmd.Printf("  ✅ 微信(wxsession)分享单曲上报成功 (SongID: %s)\n", songIdStr)
	}
}

// doVipSimulateBrowse 模拟请求活动页面并在本地进行停留随机延迟，达成网页停留要求
func (c *SignIn) doVipSimulateBrowse(ctx context.Context, cli *api.Client, request *weapi.Api, jumpUrl string, minSec, maxSec int, taskName string) {
	if jumpUrl == "" {
		c.cmd.Printf("  ⚠️ [%s] 的跳转链接为空，跳过浏览模拟\n", taskName)
		return
	}

	// Ensure jumpUrl has fromRN=1 to indicate App environment
	if !strings.Contains(jumpUrl, "fromRN=") {
		if strings.Contains(jumpUrl, "?") {
			jumpUrl += "&fromRN=1"
		} else {
			jumpUrl += "?fromRN=1"
		}
	}

	// Try parsing task parameters from jumpUrl for page view reporting
	var (
		taskId       string
		taskType     int    = 200
		viewTime     int64  = 15
		taskBusiness string = "music.vip_growth"
	)

	var pageCodes []string

	if parsedUrl, err := url.Parse(jumpUrl); err == nil {
		q := parsedUrl.Query()
		if q.Get("view_task_id") != "" {
			taskId = q.Get("view_task_id")
		}
		if q.Get("view_task_business") != "" {
			taskBusiness = q.Get("view_task_business")
		}
		if q.Get("view_time") != "" {
			if vt, err := strconv.ParseInt(q.Get("view_time"), 10, 64); err == nil {
				viewTime = vt
			}
		}
		if q.Get("view_task_type") != "" {
			if tt, err := strconv.Atoi(q.Get("view_task_type")); err == nil {
				taskType = tt
			}
		}

		// Dynamically construct pageCodes based on URL path base, component or route param
		if q.Get("component") != "" {
			comp := q.Get("component")
			cleaned := strings.ReplaceAll(comp, "-", "_")
			pageCodes = append(pageCodes, "music_vip_"+cleaned)
			pageCodes = append(pageCodes, cleaned)
			pageCodes = append(pageCodes, "music_vip_"+comp)
			pageCodes = append(pageCodes, comp)
			if strings.HasPrefix(comp, "rn-") {
				noRn := comp[3:]
				cleanedNoRn := strings.ReplaceAll(noRn, "-", "_")
				pageCodes = append(pageCodes, "music_vip_"+cleanedNoRn)
				pageCodes = append(pageCodes, cleanedNoRn)
				pageCodes = append(pageCodes, "music_vip_"+noRn)
				pageCodes = append(pageCodes, noRn)
			}
		}

		if q.Get("route") != "" {
			route := q.Get("route")
			cleanedRoute := strings.ReplaceAll(route, "-", "_")
			pageCodes = append(pageCodes, "music_vip_"+cleanedRoute)
			pageCodes = append(pageCodes, cleanedRoute)
			pageCodes = append(pageCodes, "music_vip_"+route)
			pageCodes = append(pageCodes, route)
		}

		if q.Get("component") != "" && q.Get("route") != "" {
			comp := q.Get("component")
			route := q.Get("route")
			comb := comp + "_" + route
			cleanedComb := strings.ReplaceAll(comb, "-", "_")
			pageCodes = append(pageCodes, "music_vip_"+cleanedComb)
			pageCodes = append(pageCodes, cleanedComb)
			pageCodes = append(pageCodes, "music_vip_"+comb)
			pageCodes = append(pageCodes, comb)
		}

		pathParts := strings.Split(parsedUrl.Path, "/")
		if len(pathParts) > 0 {
			lastPart := pathParts[len(pathParts)-1]
			if lastPart != "" {
				cleaned := strings.ReplaceAll(lastPart, "-", "_")
				pageCodes = append(pageCodes, "music_vip_"+cleaned)
				pageCodes = append(pageCodes, cleaned)
			}
		}
	}

	if len(pageCodes) == 0 {
		pageCodes = append(pageCodes, "music_vip_sound_effect_detail")
	}

	var webkitContext string
	webViewId := fmt.Sprintf("%d", 1000000000+rand.Int63n(9000000000))
	escapedHref := strings.ReplaceAll(jumpUrl, "/", "\\/")
	webkitContext = fmt.Sprintf(`{"webViewId":"%s","href":"%s","newebkit":1}`, webViewId, escapedHref)

	if strings.HasPrefix(jumpUrl, "http://") || strings.HasPrefix(jumpUrl, "https://") {
		c.cmd.Printf("  👉 模拟加载 [%s] 页面...\n", taskName)
		httpClient := cli.GetClient()
		req, err := http.NewRequestWithContext(ctx, "GET", jumpUrl, nil)
		if err != nil {
			c.cmd.Printf("  ⚠️ [%s] 创建请求失败: %v\n", taskName, err)
		} else {
			req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 CloudMusic/0.1.1 NeteaseMusic/9.4.95")
			req.Header.Set("netease_webkit_context", webkitContext)
			req.Header.Set("Referer", "https://music.163.com/")
			req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
			req.Header.Set("Accept-Language", "zh-CN,zh-Hans;q=0.9")

			resp, err := httpClient.Do(req)
			if err != nil {
				c.cmd.Printf("  ⚠️ [%s] 模拟页面请求失败 (but usually does not affect result): %v\n", taskName, err)
			} else {
				resp.Body.Close()
				c.cmd.Printf("  ℹ️ 页面加载响应码: %d\n", resp.StatusCode)
			}
		}
	} else {
		c.cmd.Printf("  ℹ️ [%s] 为 App 内部跳转链接，跳过 HTTP 模拟加载，直接进行浏览上报...\n", taskName)
	}

	// Report viewStart to create a server-side browsing session (for all candidate pageCodes and resourceTypes)
	if taskId != "" {
		resourceTypes := []string{"vip_growth", taskBusiness, ""}
		for _, pc := range pageCodes {
			for _, rt := range resourceTypes {
				startBytes, _ := json.Marshal(weapi.VipMiddlePageViewReportData{
					ActionType:   "viewStart",
					Time:         time.Now().UnixNano() / int64(time.Millisecond),
					TaskId:       taskId,
					TaskType:     taskType,
					ViewTime:     0,
					JumpUrl:      jumpUrl,
					TaskBusiness: taskBusiness,
					ResourceType: rt,
					PageCode:     pc,
				})
				_, _ = request.VipMiddlePageViewReport(ctx, &weapi.VipMiddlePageViewReportReq{
					WebkitContext: webkitContext,
					Data:          string(startBytes),
				})
			}
		}
	}

	// Stay at least viewTime seconds plus a little random delay
	sleepSec := int(viewTime)
	if minSec > sleepSec {
		sleepSec = minSec
	}
	if maxSec > sleepSec {
		sleepSec = sleepSec + rand.Intn(maxSec-sleepSec+1)
	} else {
		sleepSec = sleepSec + rand.Intn(5) // default 0~4s random delay
	}

	c.cmd.Printf("  ⏳ 正在模拟 [%s] 的页面停留，等待 %d 秒...\n", taskName, sleepSec)
	select {
	case <-ctx.Done():
		c.cmd.Println("  ❌ 浏览模拟被取消")
		return
	case <-time.After(time.Duration(sleepSec) * time.Second):
	}
	c.cmd.Printf("  ✅ [%s] 页面浏览模拟完成\n", taskName)

	// Report page view end to complete the task (for all candidate pageCodes and resourceTypes)
	if taskId != "" {
		resourceTypes := []string{"vip_growth", taskBusiness, ""}
		for _, pc := range pageCodes {
			for _, rt := range resourceTypes {
				c.cmd.Printf("  👉 正在上报 [%s] 的浏览完成状态 (taskId: %s, pageCode: %s, resourceType: %s, duration: %d秒)...\n", taskName, taskId, pc, rt, sleepSec)
				dataBytes, _ := json.Marshal(weapi.VipMiddlePageViewReportData{
					ActionType:   "viewEnd",
					Time:         time.Now().UnixNano() / int64(time.Millisecond),
					TaskId:       taskId,
					TaskType:     taskType,
					ViewTime:     int64(sleepSec) * 1000,
					JumpUrl:      jumpUrl,
					TaskBusiness: taskBusiness,
					ResourceType: rt,
					PageCode:     pc,
				})
				resp, reportErr := request.VipMiddlePageViewReport(ctx, &weapi.VipMiddlePageViewReportReq{
					WebkitContext: webkitContext,
					Data:          string(dataBytes),
				})
				if reportErr != nil {
					c.cmd.Printf("  ❌ 上报 [%s](%s/%s) 浏览完成状态失败: %v\n", taskName, pc, rt, reportErr)
				} else {
					c.cmd.Printf("  ✅ 上报 [%s](%s/%s) 浏览完成状态成功: code=%d, data=%v, msg=%s\n", taskName, pc, rt, resp.Code, resp.Data, resp.Message)
				}
			}
		}
	}
}

// doVipWelfareClaim 获取会员等级福利列表并自动领取第一个未领福利
func (c *SignIn) doVipWelfareClaim(ctx context.Context, request *weapi.Api, eapiRequest *eapi.Api, userLevel int64) {
	// 1. 优先尝试自动领券打卡日常“免费领福利”任务
	c.cmd.Println("  👉 获取当前常驻免费商家福利券列表...")
	benefitList, err := eapiRequest.VipBenefitCategoryList(ctx, &eapi.VipBenefitCategoryListReq{
		Category: "1291816",
		Header:   "{}",
		ER:       true,
	})
	if err == nil && benefitList.Code == 200 && len(benefitList.Data) > 0 {
		var targetBenefitId int64
		var targetBenefitName string
		// 优先查找含有“免费”或“0元”等字样的纯免费商家福利券
		for _, b := range benefitList.Data {
			if !b.BenefitGet && b.Id > 0 {
				nameLower := strings.ToLower(b.Name)
				if strings.Contains(nameLower, "免费") || strings.Contains(nameLower, "0元") {
					targetBenefitId = b.Id
					targetBenefitName = b.Name
					break
				}
			}
		}
		// 如果没有找到带免费字样的，再兜底选择第一个未领取的福利券
		if targetBenefitId == 0 {
			for _, b := range benefitList.Data {
				if !b.BenefitGet && b.Id > 0 {
					targetBenefitId = b.Id
					targetBenefitName = b.Name
					break
				}
			}
		}

		if targetBenefitId > 0 {
			c.cmd.Printf("  👉 发现尚未领取的商家福利券: [%s] (Id: %d)，开始自动领券打卡...\n", targetBenefitName, targetBenefitId)
			getResp, getErr := eapiRequest.VipBenefitGet(ctx, &eapi.VipBenefitGetReq{
				Id:     fmt.Sprintf("%d", targetBenefitId),
				Header: "{}",
				ER:     true,
			})
			if getErr != nil {
				c.cmd.Printf("  ❌ 领取福利券 [%s] 失败 (网络错误): %v\n", targetBenefitName, getErr)
			} else if getResp.Code != 200 || !getResp.Result.BenefitGet {
				c.cmd.Printf("  ❌ 领取福利券 [%s] 失败: code=%d, msg=%s\n", targetBenefitName, getResp.Code, getResp.Message)
			} else {
				c.cmd.Printf("  🎉 成功领券打卡日常福利任务: [%s]\n", targetBenefitName)
			}
		} else {
			c.cmd.Println("  ℹ️ 所有商家福利券都已领过，继续后续福利打卡")
		}
	} else {
		c.cmd.Printf("  ⚠️ 获取商家福利券列表失败 (可能是网络波动或接口微调): %v\n", err)
	}

	// 2. 兜底方案：触发 EAPI 版本的福利列表获取，此操作同样可尝试触发日常任务判定
	_, _ = eapiRequest.VipWelfareList(ctx, &eapi.VipWelfareListReq{
		Header: "{}",
		ER:     true,
	})

	c.cmd.Println("  👉 获取当前可领取的黑胶等级福利列表...")
	welfareList, err := request.VipWelfareList(ctx, &weapi.VipWelfareListReq{})
	if err != nil || welfareList.Code != 200 {
		c.cmd.Printf("  ❌ 获取福利列表失败: %v\n", err)
		return
	}

	var targetWelfareId int64
	var targetWelfareName string

	// 遍历等级 map 寻找 UserReceiveStatus 为 0 (代表未领取) 且 Id > 0 的等级特权
	for levelKey, list := range welfareList.Data {
		var level int64
		if lVal, err := strconv.ParseInt(levelKey, 10, 64); err == nil {
			level = lVal
		}
		if userLevel > 0 && level > userLevel {
			continue
		}

		for _, w := range list {
			if w.UserReceiveStatus == 0 && w.Id > 0 {
				targetWelfareId = w.Id
				targetWelfareName = fmt.Sprintf("%s (等级特权:%s)", w.ShowName, levelKey)
				break
			}
		}
		if targetWelfareId > 0 {
			break
		}
	}

	if targetWelfareId == 0 {
		c.cmd.Println("  ℹ️ 没有找到当前可领取的等级特权福利，可能已全部领取")
		return
	}

	c.cmd.Printf("  👉 发现可领取福利: [%s] (Id: %d)，开始执行领取...\n", targetWelfareName, targetWelfareId)
	claimResp, err := request.VipWelfareClaim(ctx, &weapi.VipWelfareClaimReq{
		WelfareId: targetWelfareId,
	})
	if err != nil {
		c.cmd.Printf("  ❌ 领取福利 [%s] 失败 (网络错误): %v\n", targetWelfareName, err)
	} else if claimResp.Code == 404 {
		c.cmd.Printf("  ℹ️ 福利 [%s] 无法自动领取，可能已被领完或相关活动接口已下线 (404)\n", targetWelfareName)
	} else if claimResp.Code != 200 {
		c.cmd.Printf("  ❌ 领取福利 [%s] 失败 (服务限制): code=%d, msg=%s\n", targetWelfareName, claimResp.Code, claimResp.Message)
	} else {
		c.cmd.Printf("  🎉 成功领取尊享等级福利: [%s]\n", targetWelfareName)
	}
}

// getVipProgressTasks 获取成长中心任务列表，将其转换为标准任务格式以丰富黑胶任务列表
func getVipProgressTasks(ctx context.Context, request *weapi.Api) ([]eapi.VipTaskListData, error) {
	resp, err := request.VipProgressList(ctx, &weapi.VipProgressListReq{})
	if err != nil {
		return nil, err
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("VipProgressList code=%d", resp.Code)
	}

	var list []eapi.VipTaskListData
	for _, item := range resp.Data {
		var jumpUrl string
		if item.BasicMissionDTO.SchemaContent != "" {
			var sc struct {
				JumpUrl string `json:"jumpUrl"`
			}
			_ = json.Unmarshal([]byte(item.BasicMissionDTO.SchemaContent), &sc)
			jumpUrl = sc.JumpUrl
			if jumpUrl == "" {
				var scSpace struct {
					JumpUrl string `json:"jumpUrl "`
				}
				_ = json.Unmarshal([]byte(item.BasicMissionDTO.SchemaContent), &scSpace)
				jumpUrl = scSpace.JumpUrl
			}
		}

		worth := int64(0)
		if len(item.StageProgressDTOS) > 0 {
			worth = item.StageProgressDTOS[0].Worth
		}

		list = append(list, eapi.VipTaskListData{
			MissionId:       item.BasicMissionDTO.MissionId,
			MissionCode:     item.BasicMissionDTO.MissionCode,
			MissionType:     item.BasicMissionDTO.MissionType,
			MissionEntityId: item.BasicMissionDTO.MissionEntityId,
			Status:          item.MissionStatus,
			Worth:           worth,
			MainTitle:       item.BasicMissionDTO.Name,
			JumpUrl:         jumpUrl,
		})
	}
	return list, nil
}
