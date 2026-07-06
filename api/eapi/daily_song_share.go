// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package eapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/3899/ncmm/api"
)

const dailySongShareMConfigInfo = `{"IuRPVVmc3WWul9fT":{"version":"115240960","appver":"9.5.37"},"tPJJnts2H31BZXmp":{"version":"5230592","appver":"4.74.0"},"c0Ve6C0uNl2Am0Rl":{"version":"276480","appver":"1.4.30"},"zr4bw6pKFDIZScpo":{"version":"3758080","appver":"2.40.0"}}`

type DailySongShareBaseReq struct {
	Header string `json:"header"`
	ER     bool   `json:"e_r"`
}

func (r *DailySongShareBaseReq) fill() {
	if r.Header == "" {
		r.Header = "{}"
	}
	r.ER = true
}

type DailySongShareRegisterReq struct {
	DailySongShareBaseReq
}

type DailySongShareRegisterResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		NoteAttendance bool `json:"noteAttendance"`
	} `json:"data"`
}

func (a *Api) DailySongShareRegister(ctx context.Context, req *DailySongShareRegisterReq) (*DailySongShareRegisterResp, error) {
	if req == nil {
		req = &DailySongShareRegisterReq{}
	}
	req.fill()
	var (
		url   = "https://interface3.music.163.com/xeapi/note/common/activity/in/registration"
		reply DailySongShareRegisterResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeXEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type DailySongShareRegistrationGuideReq struct {
	DailySongShareBaseReq
}

type DailySongShareRegistrationGuideResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		RegisterStatus  string `json:"registerStatus"`
		ActivityId      int64  `json:"activityId"`
		ActivityCycleId int64  `json:"activityCycleId"`
		// ActivityInterestId is the lottery activity id used by middle/play/do/lottery.
		ActivityInterestId int64  `json:"activityInterestId"`
		RewardJumpUrl      string `json:"rewardJumpUrl"`
		Duration           string `json:"duration"`
		RegisteredGuide    struct {
			Title           string `json:"title"`
			SignUp          string `json:"signUp"`
			SignTip         string `json:"signTip"`
			RewardCount     int    `json:"rewardCount"`
			HaveRewardCount int    `json:"haveRewardCount"`
			AlreadyPubEvent bool   `json:"alreadyPubEvent"`
			PubEventCount   int    `json:"pubEventCount"`
		} `json:"noteAttendanceRegisteredGuideVo"`
	} `json:"data"`
}

func (a *Api) DailySongShareRegistrationGuide(ctx context.Context, req *DailySongShareRegistrationGuideReq) (*DailySongShareRegistrationGuideResp, error) {
	if req == nil {
		req = &DailySongShareRegistrationGuideReq{}
	}
	req.fill()
	var (
		url   = "https://interface3.music.163.com/xeapi/note/attendance/activity/registration/v2/guide"
		reply DailySongShareRegistrationGuideResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeXEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type DailySongSharePublishReq struct {
	OS                 string `json:"os"`
	AddComment         bool   `json:"addComment"`
	AutoSaveDraft      bool   `json:"autoSaveDraft"`
	Id                 string `json:"id,omitempty"`
	Msg                string `json:"msg"`
	SessionId          string `json:"sessionId"`
	TargetPublishTime  string `json:"targetPublishTime"`
	ServerUuid         string `json:"serverUuid"`
	UseNewUpload       bool   `json:"useNewUpload"`
	FromRN             bool   `json:"fromRN"`
	ActivityInfoList   string `json:"activityInfoList,omitempty"`
	PubSource          string `json:"pubSource"`
	PubTraceId         string `json:"pubTraceId"`
	PublishTime        string `json:"publishTime"`
	Title              string `json:"title,omitempty"`
	SocialSpaceVisible int    `json:"socialSpaceVisible"`
	PrivacySetting     string `json:"privacySetting"`
	UID                string `json:"uid"`
	ContainAiContent   bool   `json:"containAiContent"`
	Uuid               string `json:"uuid"`
	Type               string `json:"type"`
	Pics               string `json:"pics,omitempty"`
	NeedsGuardianToken bool   `json:"needsGuardianToken"`
	CheckToken         string `json:"checkToken,omitempty"`
	Header             string `json:"header"`
	ER                 bool   `json:"e_r"`
}

func (a *Api) DailySongSharePublish(ctx context.Context, req *DailySongSharePublishReq) (*EventPublishResp, error) {
	if err := fillDailySongSharePublishDefaults(req); err != nil {
		return nil, err
	}
	var (
		url   = "https://interface3.music.163.com/xeapi/note/share/friends/resource"
		reply EventPublishResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeXEAPI
	opts.SetHeader("cm_no_encrypt_native_tag_20220105", "false")
	opts.SetHeader("CMPageId", "page_songlist")
	opts.SetHeader("MConfig-Info", dailySongShareMConfigInfo)
	a.fillDailySongShareAntiCheatToken(req, opts)
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

func fillDailySongSharePublishDefaults(req *DailySongSharePublishReq) error {
	if req == nil {
		return fmt.Errorf("daily song share publish request is nil")
	}
	if req.OS == "" {
		req.OS = "android"
	}
	req.AutoSaveDraft = true
	if req.SessionId == "" {
		sessionId, err := randomDailySongSessionID()
		if err != nil {
			return err
		}
		req.SessionId = sessionId
	}
	if req.TargetPublishTime == "" {
		req.TargetPublishTime = "-1"
	}
	if req.ServerUuid == "" {
		serverUuid, err := randomDailySongHex(16, true)
		if err != nil {
			return err
		}
		req.ServerUuid = serverUuid
	}
	req.UseNewUpload = true
	req.FromRN = true
	if req.PubTraceId == "" {
		pubTraceId, err := randomDailySongHex(16, true)
		if err != nil {
			return err
		}
		req.PubTraceId = pubTraceId
	}
	if req.PublishTime == "" {
		req.PublishTime = "0"
	}
	if req.PrivacySetting == "" {
		req.PrivacySetting = "0"
	}
	if req.SocialSpaceVisible == 0 {
		req.SocialSpaceVisible = 1
	}
	if req.Uuid == "" {
		uuid, err := randomDailySongHex(16, true)
		if err != nil {
			return err
		}
		req.Uuid = uuid
	}
	if req.Type == "" {
		req.Type = "noresource"
	}
	req.NeedsGuardianToken = true
	if req.Header == "" {
		req.Header = "{}"
	}
	req.ER = true
	return nil
}

func (a *Api) fillDailySongShareAntiCheatToken(req *DailySongSharePublishReq, opts *api.Options) {
	token := strings.TrimSpace(req.CheckToken)
	if token == "" {
		return
	}
	req.CheckToken = token
	req.ER = true
	if req.Header == "" {
		req.Header = "{}"
	}
	opts.SetHeader("X-antiCheatToken", token)
	if musicU := a.client.MusicU(); musicU != "" {
		opts.SetHeader("x-music-u", musicU)
	}
}

func dailySongShareAntiCheatHeader(token string) string {
	data, err := json.Marshal(map[string]string{
		"X-antiCheatToken": token,
	})
	if err != nil {
		return "{}"
	}
	return string(data)
}

func randomDailySongSessionID() (string, error) {
	value, err := randomDailySongHex(6, false)
	if err != nil {
		return "", err
	}
	return value[:8] + "-" + value[8:11], nil
}

func randomDailySongHex(size int, upper bool) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate daily song random id: %w", err)
	}
	value := hex.EncodeToString(b)
	if upper {
		value = strings.ToUpper(value)
	}
	return value, nil
}

type DailySongShareLotteryReq struct {
	ActivityId int64  `json:"activityId"`
	Header     string `json:"header"`
	ER         bool   `json:"e_r"`
	CheckToken string `json:"checkToken,omitempty"`
}

type DailySongShareLotteryPrizeDetail struct {
	PrizeName    string   `json:"prizeName"`
	WinPrizeDesc string   `json:"winPrizeDesc"`
	PrizeImgList []string `json:"prizeImgList"`
	ExchangeUrl  string   `json:"exchangeUrl"`
	PrizeType    int      `json:"prizeType"`
	SubType      int      `json:"subType"`
	ContentId    string   `json:"contentId"`
	DefaultPrize int      `json:"defaultPrize"`
	PrizeLevel   int      `json:"prizeLevel"`
}

type DailySongShareLotteryResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		UserId             int64                                       `json:"userId"`
		IdempotentId       string                                      `json:"idempotentId"`
		ActivityId         int64                                       `json:"activityId"`
		PrizeSchemeId      int64                                       `json:"prizeSchemeId"`
		DrawPrizeTime      int64                                       `json:"drawPrizeTime"`
		PrizeDetailInfoMap map[string]DailySongShareLotteryPrizeDetail `json:"prizeDetailInfoMap"`
		NoLotteryContent   interface{}                                 `json:"noLotteryContent"`
		RestChance         int                                         `json:"restChance"`
	} `json:"data"`
}

func (a *Api) DailySongShareLottery(ctx context.Context, req *DailySongShareLotteryReq) (*DailySongShareLotteryResp, error) {
	if req == nil {
		req = &DailySongShareLotteryReq{}
	}
	if req.Header == "" {
		req.Header = "{}"
	}
	req.ER = true
	var (
		url   = "https://interface3.music.163.com/xeapi/middle/play/do/lottery"
		reply DailySongShareLotteryResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeXEAPI
	token := strings.TrimSpace(req.CheckToken)
	if token != "" {
		req.CheckToken = token
		if req.Header == "" || req.Header == "{}" {
			req.Header = dailySongShareAntiCheatHeader(token)
		}
		opts.SetHeader("X-antiCheatToken", token)
		if musicU := a.client.MusicU(); musicU != "" {
			opts.SetHeader("x-music-u", musicU)
		}
	}
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}
