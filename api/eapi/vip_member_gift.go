// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package eapi

import (
	"context"
	"fmt"
	neturl "net/url"
	"strings"

	"github.com/3899/ncmm/api"
)

type VipMemberGiftBaseReq struct {
	Header     interface{}    `json:"header"`
	ER         bool           `json:"e_r"`
	DeviceID   string         `json:"deviceId,omitempty"`
	OS         string         `json:"os,omitempty"`
	VerifyID   int            `json:"verifyId,omitempty"`
	CryptoMode api.CryptoMode `json:"-"`
}

func (r *VipMemberGiftBaseReq) fill(client *api.Client) api.CryptoMode {
	mode := r.CryptoMode
	if mode == "" {
		mode = api.CryptoModeXEAPI
	}
	if mode == api.CryptoModeEAPI {
		if vipMemberGiftHeaderEmpty(r.Header) {
			r.Header = map[string]interface{}{}
		}
		if strings.TrimSpace(r.DeviceID) == "" {
			r.DeviceID = firstNonEmpty(
				vipMemberGiftCookieValue(client, "deviceId"),
				vipMemberGiftCookieValue(client, "sDeviceId", "sdeviceId"),
				client.GetDeviceId(),
			)
		}
		if strings.TrimSpace(r.OS) == "" {
			r.OS = "iOS"
		}
		if r.VerifyID == 0 {
			r.VerifyID = 1
		}
	} else if vipMemberGiftHeaderEmpty(r.Header) {
		r.Header = "{}"
	}
	r.ER = true
	return mode
}

type VipMemberGiftTokenCreateReq struct {
	VipMemberGiftBaseReq
}

type VipMemberGiftTokenCreateResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Token string `json:"token"`
	} `json:"data"`
}

func (a *Api) VipMemberGiftTokenCreate(ctx context.Context, req *VipMemberGiftTokenCreateReq) (*VipMemberGiftTokenCreateResp, error) {
	if req == nil {
		req = &VipMemberGiftTokenCreateReq{}
	}
	mode := req.fill(a.client)
	var (
		url   = vipMemberGiftURL(mode, "token/create")
		reply VipMemberGiftTokenCreateResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = mode
	a.applyVipMemberGiftHeaders(opts, mode)
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipMemberGiftPageInfoReq struct {
	VipMemberGiftBaseReq
}

type VipMemberGiftPageInfoResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		CanSendVip            int    `json:"canSendVip"`
		SendVipDayTotal       int    `json:"sendVipDayTotal"`
		SendVipDayLeft        int    `json:"sendVipDayLeft"`
		ButtonText            string `json:"buttonText"`
		Desc                  string `json:"desc"`
		VipLevelDesc          string `json:"vipLevelDesc"`
		Img                   string `json:"img"`
		VipLevel              int    `json:"vipLevel"`
		HasSendVipInThisMonth bool   `json:"hasSendVipInThisMonth"`
	} `json:"data"`
}

func (a *Api) VipMemberGiftPageInfo(ctx context.Context, req *VipMemberGiftPageInfoReq) (*VipMemberGiftPageInfoResp, error) {
	if req == nil {
		req = &VipMemberGiftPageInfoReq{}
	}
	mode := req.fill(a.client)
	var (
		url   = vipMemberGiftURL(mode, "page/info")
		reply VipMemberGiftPageInfoResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = mode
	a.applyVipMemberGiftHeaders(opts, mode)
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipMemberGiftDetailReq struct {
	VipMemberGiftBaseReq
	Token    string `json:"token,omitempty"`
	RecordID int64  `json:"recordId,omitempty"`
}

type VipMemberGiftUser struct {
	UserID    int64  `json:"userId"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatarUrl"`
}

type VipMemberGiftDetailResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		RecordID             int64              `json:"recordId"`
		Inviter              *VipMemberGiftUser `json:"inviter"`
		Invitee              *VipMemberGiftUser `json:"invitee"`
		TokenExpireTime      int64              `json:"tokenExpireTime"`
		AcceptedRewardImgURL string             `json:"accpetedRewardImgUrl"`
		DetailRewardImgURL   string             `json:"detailRewardImgUrl"`
		RewardName           string             `json:"rewardName"`
		ShareSwitch          bool               `json:"shareSwitch"`
		CurrentUserRecordID  *int64             `json:"currentUserRecordId"`
		InviterTotalDays     *int64             `json:"inviterTotalDays"`
		VipType              int                `json:"vipType"`
		Duration             int                `json:"duration"`
		AcceptTime           int64              `json:"acceptTime"`
	} `json:"data"`
}

func (a *Api) VipMemberGiftDetail(ctx context.Context, req *VipMemberGiftDetailReq) (*VipMemberGiftDetailResp, error) {
	if req == nil {
		req = &VipMemberGiftDetailReq{}
	}
	mode := req.fill(a.client)
	var (
		url   = vipMemberGiftURL(mode, "detail/info/get")
		reply VipMemberGiftDetailResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = mode
	a.applyVipMemberGiftHeaders(opts, mode)
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipMemberGiftAcceptReq struct {
	VipMemberGiftBaseReq
	Token      string `json:"token"`
	Refer      string `json:"refer,omitempty"`
	CheckToken string `json:"checkToken,omitempty"`
}

type VipMemberGiftAcceptResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		RecordID             int64              `json:"recordId"`
		AcceptedRewardImgURL string             `json:"accpetedRewardImgUrl"`
		ActivityRewardImgURL string             `json:"activityRewardImgUrl"`
		VipType              int                `json:"vipType"`
		RewardName           string             `json:"rewardName"`
		DetailRewardImgURL   string             `json:"detailRewardImgUrl"`
		CanReceiveCoupon     bool               `json:"canReceiveCoupon"`
		Inviter              *VipMemberGiftUser `json:"inviter"`
		Duration             int                `json:"duration"`
		CouponImageURL       string             `json:"couponImageUrl"`
		VipLevel             int                `json:"vipLevel"`
	} `json:"data"`
}

func (a *Api) VipMemberGiftAccept(ctx context.Context, req *VipMemberGiftAcceptReq) (*VipMemberGiftAcceptResp, error) {
	if req == nil {
		return nil, fmt.Errorf("vip member gift accept request is nil")
	}
	mode := req.fill(a.client)
	var (
		url   = vipMemberGiftURL(mode, "accept")
		reply VipMemberGiftAcceptResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = mode
	a.applyVipMemberGiftHeaders(opts, mode)
	token := strings.TrimSpace(req.CheckToken)
	if token != "" {
		req.CheckToken = token
		if mode == api.CryptoModeXEAPI && vipMemberGiftHeaderEmpty(req.Header) {
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

func vipMemberGiftURL(mode api.CryptoMode, path string) string {
	prefix := "xeapi"
	if mode == api.CryptoModeEAPI {
		prefix = "eapi"
	}
	return "https://interface3.music.163.com/" + prefix + "/vipactivity/app/vip/invitation/" + path
}

func (a *Api) applyVipMemberGiftHeaders(opts *api.Options, mode api.CryptoMode) {
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

func vipMemberGiftIOSUserAgent(client *api.Client) string {
	ua := strings.TrimSpace(client.UserAgent(api.CryptoModeEAPI))
	if vipMemberGiftPlatformFromText(ua) == "ios" {
		return ua
	}
	appver := vipMemberGiftCookieValue(client, "appver")
	osver := vipMemberGiftCookieValue(client, "osver")
	if appver != "" && osver != "" {
		device := "iPhone"
		if strings.Contains(strings.ToLower(vipMemberGiftCookieValue(client, "os")), "ipad") {
			device = "iPad"
		}
		return fmt.Sprintf("neteasemusic/%s (%s; iOS %s; Scale/2.00)", appver, device, osver)
	}
	return ua
}

func vipMemberGiftCookieValue(client *api.Client, names ...string) string {
	for _, baseURL := range []string{"https://music.163.com", "https://interface3.music.163.com"} {
		for _, name := range names {
			if ck, ok := client.Cookie(baseURL, name); ok && strings.TrimSpace(ck.Value) != "" {
				value, err := neturl.QueryUnescape(ck.Value)
				if err != nil {
					value = ck.Value
				}
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func vipMemberGiftHeaderEmpty(header interface{}) bool {
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

func vipMemberGiftPlatformFromText(value string) string {
	value = strings.ToLower(value)
	switch {
	case strings.Contains(value, "android"):
		return "android"
	case strings.Contains(value, "iphone"), strings.Contains(value, "ipad"), strings.Contains(value, "ios"):
		return "ios"
	default:
		return ""
	}
}
