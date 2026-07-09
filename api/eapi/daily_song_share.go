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
	Header     interface{}    `json:"header"`
	ER         bool           `json:"e_r"`
	DeviceID   string         `json:"deviceId,omitempty"`
	OS         string         `json:"os,omitempty"`
	VerifyID   int            `json:"verifyId,omitempty"`
	CryptoMode api.CryptoMode `json:"-"`
}

func (r *DailySongShareBaseReq) fill(client *api.Client) api.CryptoMode {
	mode := r.CryptoMode
	if mode == "" {
		mode = api.CryptoModeXEAPI
	}
	if mode == api.CryptoModeEAPI {
		fillDailySongShareIOSBase(client, &r.Header, &r.DeviceID, &r.OS, &r.VerifyID)
	} else if dailySongShareHeaderEmpty(r.Header) {
		r.Header = "{}"
	}
	r.ER = true
	return mode
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
	mode := req.fill(a.client)
	var (
		url   = dailySongShareURL(mode, "note/common/activity/in/registration")
		reply DailySongShareRegisterResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = mode
	a.applyDailySongShareHeaders(opts, mode)
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type DailySongShareAttendanceRegisterReq struct {
	DailySongShareBaseReq
	ActivityId      int64 `json:"activityId"`
	ActivityCycleId int64 `json:"activityCycleId"`
	AutoRegister    bool  `json:"autoRegister"`
}

type DailySongShareAttendanceRegisterResp struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

func (a *Api) DailySongShareAttendanceRegister(ctx context.Context, req *DailySongShareAttendanceRegisterReq) (*DailySongShareAttendanceRegisterResp, error) {
	if req == nil {
		return nil, fmt.Errorf("daily song share attendance register request is nil")
	}
	req.AutoRegister = true
	mode := req.fill(a.client)
	var (
		url   = dailySongShareURL(mode, "note/attendance/activity/register")
		reply DailySongShareAttendanceRegisterResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = mode
	a.applyDailySongShareHeaders(opts, mode)
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
	mode := req.fill(a.client)
	var (
		url   = dailySongShareURL(mode, "note/attendance/activity/registration/v2/guide")
		reply DailySongShareRegistrationGuideResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = mode
	a.applyDailySongShareHeaders(opts, mode)
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type DailySongSharePublishReq struct {
	OS                 string         `json:"os"`
	AddComment         bool           `json:"addComment"`
	AutoSaveDraft      bool           `json:"autoSaveDraft"`
	Id                 string         `json:"id,omitempty"`
	ThreadID           string         `json:"threadId,omitempty"`
	ResourceID         string         `json:"resourceId,omitempty"`
	Msg                string         `json:"msg"`
	SessionId          string         `json:"sessionId"`
	TargetPublishTime  interface{}    `json:"targetPublishTime"`
	ServerUuid         string         `json:"serverUuid"`
	UseNewUpload       bool           `json:"useNewUpload"`
	FromRN             bool           `json:"fromRN"`
	ActivityInfoList   string         `json:"activityInfoList,omitempty"`
	PubSource          string         `json:"pubSource"`
	PubTraceId         string         `json:"pubTraceId"`
	PublishTime        interface{}    `json:"publishTime"`
	Title              string         `json:"title,omitempty"`
	SocialSpaceVisible int            `json:"socialSpaceVisible"`
	PrivacySetting     string         `json:"privacySetting"`
	UID                string         `json:"uid"`
	ContainAiContent   bool           `json:"containAiContent"`
	Uuid               string         `json:"uuid"`
	Type               string         `json:"type"`
	Pics               string         `json:"pics,omitempty"`
	NeedsGuardianToken bool           `json:"needsGuardianToken"`
	CheckToken         string         `json:"checkToken,omitempty"`
	Header             interface{}    `json:"header"`
	DeviceID           string         `json:"deviceId,omitempty"`
	VerifyID           int            `json:"verifyId,omitempty"`
	ContentDeclaration string         `json:"contentDeclaration,omitempty"`
	ProduceInfo        string         `json:"produceInfo,omitempty"`
	RepostSource       string         `json:"repostSource,omitempty"`
	ER                 bool           `json:"e_r"`
	CryptoMode         api.CryptoMode `json:"-"`
}

func (a *Api) DailySongSharePublish(ctx context.Context, req *DailySongSharePublishReq) (*EventPublishResp, error) {
	mode, err := fillDailySongSharePublishDefaults(a.client, req)
	if err != nil {
		return nil, err
	}
	var (
		url   = dailySongShareURL(mode, "note/share/friends/resource")
		reply EventPublishResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = mode
	if mode == api.CryptoModeXEAPI {
		opts.SetHeader("cm_no_encrypt_native_tag_20220105", "false")
		opts.SetHeader("CMPageId", "page_songlist")
		opts.SetHeader("MConfig-Info", dailySongShareMConfigInfo)
	}
	a.applyDailySongShareHeaders(opts, mode)
	a.fillDailySongShareAntiCheatToken(req, opts)
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

func fillDailySongSharePublishDefaults(client *api.Client, req *DailySongSharePublishReq) (api.CryptoMode, error) {
	if req == nil {
		return "", fmt.Errorf("daily song share publish request is nil")
	}
	mode := req.CryptoMode
	if mode == "" {
		mode = api.CryptoModeXEAPI
	}
	req.CryptoMode = mode
	if req.OS == "" {
		if mode == api.CryptoModeEAPI {
			req.OS = "iOS"
		} else {
			req.OS = "android"
		}
	}
	req.AutoSaveDraft = true
	if req.SessionId == "" {
		sessionId, err := randomDailySongSessionID()
		if err != nil {
			return "", err
		}
		req.SessionId = sessionId
	}
	if req.TargetPublishTime == nil {
		if mode == api.CryptoModeEAPI {
			req.TargetPublishTime = -1
		} else {
			req.TargetPublishTime = "-1"
		}
	}
	if req.ServerUuid == "" {
		serverUuid, err := randomDailySongHex(16, true)
		if err != nil {
			return "", err
		}
		req.ServerUuid = serverUuid
	}
	req.UseNewUpload = true
	req.FromRN = true
	if req.PubTraceId == "" {
		pubTraceId, err := randomDailySongHex(16, true)
		if err != nil {
			return "", err
		}
		req.PubTraceId = pubTraceId
	}
	if req.PublishTime == nil {
		if mode == api.CryptoModeEAPI {
			req.PublishTime = 0
		} else {
			req.PublishTime = "0"
		}
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
			return "", err
		}
		req.Uuid = uuid
	}
	if req.Type == "" {
		req.Type = "noresource"
	}
	if req.Type == "song" && req.Id != "" {
		if req.ThreadID == "" {
			req.ThreadID = "R_SO_4_" + req.Id
		}
		if req.ResourceID == "" {
			req.ResourceID = req.Id
		}
	}
	req.NeedsGuardianToken = true
	if mode == api.CryptoModeEAPI {
		fillDailySongShareIOSBase(client, &req.Header, &req.DeviceID, &req.OS, &req.VerifyID)
	} else if dailySongShareHeaderEmpty(req.Header) {
		req.Header = "{}"
	}
	req.ER = true
	return mode, nil
}

func (a *Api) fillDailySongShareAntiCheatToken(req *DailySongSharePublishReq, opts *api.Options) {
	token := strings.TrimSpace(req.CheckToken)
	if token == "" {
		return
	}
	req.CheckToken = token
	req.ER = true
	if dailySongShareHeaderEmpty(req.Header) {
		if req.CryptoMode == api.CryptoModeEAPI {
			req.Header = map[string]interface{}{}
		} else {
			req.Header = "{}"
		}
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

type DailySongShareTriggerReq struct {
	DailySongShareBaseReq
	SongID  string `json:"songId"`
	Channel string `json:"channel"`
}

type DailySongShareTriggerResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    bool   `json:"data"`
}

func (a *Api) DailySongShareTrigger(ctx context.Context, req *DailySongShareTriggerReq) (*DailySongShareTriggerResp, error) {
	if req == nil {
		return nil, fmt.Errorf("daily song share trigger request is nil")
	}
	if strings.TrimSpace(req.Channel) == "" {
		req.Channel = "cloudmusic"
	}
	mode := req.fill(a.client)
	var (
		url   = dailySongShareURL(mode, "music/song/share/trigger")
		reply DailySongShareTriggerResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = mode
	a.applyDailySongShareHeaders(opts, mode)
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type DailySongShareLotteryReq struct {
	DailySongShareBaseReq
	ActivityId int64  `json:"activityId"`
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
	mode := req.fill(a.client)
	var (
		url   = dailySongShareURL(mode, "middle/play/do/lottery")
		reply DailySongShareLotteryResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = mode
	a.applyDailySongShareHeaders(opts, mode)
	token := strings.TrimSpace(req.CheckToken)
	if token != "" {
		req.CheckToken = token
		if mode == api.CryptoModeXEAPI && dailySongShareHeaderEmpty(req.Header) {
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

func dailySongShareURL(mode api.CryptoMode, path string) string {
	prefix := "xeapi"
	if mode == api.CryptoModeEAPI {
		prefix = "eapi"
	}
	return "https://interface3.music.163.com/" + prefix + "/" + path
}

func (a *Api) applyDailySongShareHeaders(opts *api.Options, mode api.CryptoMode) {
	if mode != api.CryptoModeEAPI {
		return
	}
	setDefault := func(key, value string) {
		if strings.TrimSpace(value) != "" && strings.TrimSpace(opts.Headers[key]) == "" {
			opts.SetHeader(key, value)
		}
	}
	os := vipMemberGiftCookieValue(a.client, "os")
	if os == "" {
		if strings.Contains(strings.ToLower(a.client.UserAgent(api.CryptoModeEAPI)), "ipad") {
			os = "iPad"
		} else {
			os = "iPhone OS"
		}
	}
	setDefault("x-os", os)
	setDefault("x-osver", vipMemberGiftCookieValue(a.client, "osver"))
	setDefault("x-appver", vipMemberGiftCookieValue(a.client, "appver"))
	setDefault("x-buildver", vipMemberGiftCookieValue(a.client, "buildver"))
	setDefault("x-deviceid", firstNonEmpty(
		vipMemberGiftCookieValue(a.client, "deviceId"),
		a.client.GetDeviceId(),
	))
	setDefault("x-sdeviceid", vipMemberGiftCookieValue(a.client, "sDeviceId", "sdeviceId"))
	setDefault("x-music-u", a.client.MusicU())
	setDefault("User-Agent", vipMemberGiftIOSUserAgent(a.client))
}

func fillDailySongShareIOSBase(client *api.Client, header *interface{}, deviceID, osName *string, verifyID *int) {
	if dailySongShareHeaderEmpty(*header) {
		*header = map[string]interface{}{}
	}
	if strings.TrimSpace(*deviceID) == "" {
		*deviceID = firstNonEmpty(
			vipMemberGiftCookieValue(client, "deviceId"),
			vipMemberGiftCookieValue(client, "sDeviceId", "sdeviceId"),
			client.GetDeviceId(),
		)
	}
	if strings.TrimSpace(*osName) == "" {
		*osName = "iOS"
	}
	if *verifyID == 0 {
		*verifyID = 1
	}
}

func dailySongShareHeaderEmpty(header interface{}) bool {
	switch v := header.(type) {
	case nil:
		return true
	case string:
		value := strings.TrimSpace(v)
		return value == "" || value == "{}"
	case map[string]interface{}:
		return len(v) == 0
	default:
		return false
	}
}
