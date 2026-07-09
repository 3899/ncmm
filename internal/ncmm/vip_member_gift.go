// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package ncmm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/3899/ncmm/api"
	"github.com/3899/ncmm/api/eapi"
	"github.com/3899/ncmm/api/weapi"
	"github.com/3899/ncmm/config"
	"github.com/3899/ncmm/pkg/database"
	"github.com/3899/ncmm/pkg/log"

	"github.com/spf13/cobra"
)

const vipMemberGiftCloudSource = "ncmm-vip-member-gift"

type vipMemberGiftProtocol struct {
	Mode     api.CryptoMode
	Platform string
	CookieOS string
}

type VipMemberGiftOpts struct {
	CookieFile string
}

type VipMemberGift struct {
	root *Root
	cmd  *cobra.Command
	l    *log.Logger
	rng  *rand.Rand
	opts VipMemberGiftOpts
}

type vipMemberGiftCloudClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type vipMemberGiftCloudEnvelope[T any] struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
	Data  T      `json:"data"`
}

type vipMemberGiftCloudToken struct {
	TokenHash     string `json:"token_hash"`
	Token         string `json:"token"`
	DonorUID      string `json:"donor_uid"`
	Month         string `json:"month"`
	TotalDays     int    `json:"total_days"`
	AvailableDays int    `json:"available_days"`
	ClaimedDays   int    `json:"claimed_days"`
	ClaimCount    int    `json:"claim_count"`
	ExpireTimeMS  int64  `json:"expire_time_ms"`
	Status        string `json:"status"`
}

type vipMemberGiftCloudClaim struct {
	Month        string `json:"month"`
	ReceiverUID  string `json:"receiver_uid"`
	TokenHash    string `json:"token_hash"`
	RecordID     string `json:"record_id"`
	Duration     int    `json:"duration"`
	RewardName   string `json:"reward_name"`
	AcceptedAtMS int64  `json:"accepted_at_ms"`
}

type vipMemberGiftCloudUpsertData struct {
	TokenHash string                   `json:"tokenHash"`
	Token     *vipMemberGiftCloudToken `json:"token"`
}

type vipMemberGiftCloudAvailableData struct {
	Claimed bool                     `json:"claimed"`
	Claim   *vipMemberGiftCloudClaim `json:"claim"`
	Token   *vipMemberGiftCloudToken `json:"token"`
}

type vipMemberGiftCloudClaimSuccessData struct {
	Duplicate bool                     `json:"duplicate"`
	Claim     *vipMemberGiftCloudClaim `json:"claim"`
	Token     *vipMemberGiftCloudToken `json:"token"`
}

type vipMemberGiftCloudStatusData struct {
	Claimed bool                     `json:"claimed"`
	Claim   *vipMemberGiftCloudClaim `json:"claim"`
}

func NewVipMemberGift(root *Root, l *log.Logger) *VipMemberGift {
	c := &VipMemberGift{
		root: root,
		l:    l,
		rng:  rand.New(rand.NewSource(time.Now().UnixNano())),
		cmd: &cobra.Command{
			Use:     "vip-member-gift",
			Short:   "[need login] Publish or claim VIP member gift invitations",
			Example: "  ncmm vip-member-gift --cookie-file run/cookie.json",
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context())
	}
	return c
}

func (c *VipMemberGift) Command() *cobra.Command {
	return c.cmd
}

func (c *VipMemberGift) addFlags() {
	c.cmd.Flags().StringVar(&c.opts.CookieFile, "cookie-file", "", "cookie file path")
}

func (c *VipMemberGift) getConfig() (*config.VipMemberGiftConf, error) {
	if c.root.Cfg.VipMemberGift != nil {
		return c.root.Cfg.VipMemberGift, nil
	}
	return nil, fmt.Errorf("vipMemberGift configuration is not set in config.yaml")
}

func (c *VipMemberGift) execute(ctx context.Context) error {
	cfg, err := c.getConfig()
	if err != nil {
		return err
	}
	if !cfg.EnableGift && !cfg.EnableClaim {
		c.cmd.Println("[vip-member-gift] skipped: enableGift and enableClaim are both false")
		return nil
	}
	if err := c.validateCommonPrerequisites(cfg); err != nil {
		c.cmd.Printf("[vip-member-gift] skipped: %v\n", err)
		return nil
	}

	var queue []string
	if c.opts.CookieFile != "" {
		queue = append(queue, c.opts.CookieFile)
	} else {
		if c.root.Cfg.Accounts == nil {
			c.cmd.Println("[vip-member-gift] no accounts configured, please check config.yaml")
			return nil
		}
		if cfg.EnableMain && c.root.Cfg.Accounts.Main != "" {
			queue = append(queue, c.root.Cfg.Accounts.Main)
		}
		if cfg.EnableSecondaries && len(c.root.Cfg.Accounts.Secondary) > 0 {
			for _, sec := range c.root.Cfg.Accounts.Secondary {
				if sec != "" {
					queue = append(queue, sec)
				}
			}
		}
	}

	if len(queue) == 0 {
		c.cmd.Println("[vip-member-gift] no available accounts, please check config.yaml")
		return nil
	}

	for i, cookieFile := range queue {
		c.cmd.Printf("[vip-member-gift] start account (%s)\n", cookieFile)
		if err := c.ExecuteForCookie(ctx, cookieFile); err != nil {
			c.cmd.Printf("[vip-member-gift] account failed (%s): %v\n", cookieFile, err)
		}
		c.cmd.Println("[vip-member-gift] --------------------------------------------------")
		if i < len(queue)-1 {
			c.sleepBetweenAccounts(ctx, cookieFile)
		}
	}
	return nil
}

func (c *VipMemberGift) sleepBetweenAccounts(ctx context.Context, currentAccount string) {
	sleepSec := 5 + c.rng.Intn(16)
	c.cmd.Printf("[vip-member-gift] account (%s) done, wait %d seconds before next account...\n", currentAccount, sleepSec)
	select {
	case <-ctx.Done():
	case <-time.After(time.Duration(sleepSec) * time.Second):
	}
}

func (c *VipMemberGift) ExecuteForCookie(ctx context.Context, cookieFile string) error {
	cfg, err := c.getConfig()
	if err != nil {
		return err
	}
	if err := c.validateCommonPrerequisites(cfg); err != nil {
		return err
	}

	networkCfg := c.root.Cfg.Network
	absPath, err := filepath.Abs(cookieFile)
	if err != nil {
		return fmt.Errorf("resolve cookie path failed: %w", err)
	}

	cfgCopy := *networkCfg
	cfgCopy.Cookie.Filepath = absPath
	cli, err := api.NewClient(&cfgCopy, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer cli.Close(ctx)

	eapiCli := eapi.New(cli)
	weapiCli := weapi.New(cli)
	user, err := weapiCli.GetUserInfo(ctx, &weapi.GetUserInfoReq{})
	if err != nil {
		return fmt.Errorf("verify login failed: %w", err)
	}
	if user.Code != 200 || user.Profile == nil || user.Account == nil {
		return fmt.Errorf("user not logged in or session expired")
	}
	protocol, err := c.detectProtocol(cli, &cfgCopy)
	if err != nil {
		return err
	}
	syncSessionConfig(ctx, cli, cookieFile, user.Account.Id, nil, c.root.Cfg.Database)
	c.cmd.Printf("[vip-member-gift] 当前账号：uid=%v 昵称=%q\n", user.Account.Id, user.Profile.Nickname)
	c.cmd.Printf("[vip-member-gift] 接口模式：%s (%s)\n", protocol.Mode, protocol.Platform)

	cloud := newVipMemberGiftCloudClient(cfg.Cloud.BaseURL, cfg.Cloud.Token)
	var firstErr error
	db, dbErr := database.NewWithOptions(c.root.Cfg.Database, 1, 0, true)
	if dbErr != nil {
		c.cmd.Printf("[vip-member-gift] 本地缓存未启用: %v\n", dbErr)
	}
	if db != nil {
		defer db.Close(ctx)
	}

	month := currentVipMemberGiftMonth()
	if cfg.EnableGift {
		if c.isLocalSuccessCached(ctx, db, user.Account.Id, month, "gift") {
			c.cmd.Printf("[vip-member-gift] 跳过赠送：%s已赠送过。\n", month)
		} else if ok, err := c.runGift(ctx, eapiCli, weapiCli, cloud, cfg, user.Account.Id, protocol); err != nil {
			c.cmd.Printf("[vip-member-gift] 赠送失败: %v\n", err)
			firstErr = err
		} else if ok {
			c.saveLocalSuccessCache(ctx, db, user.Account.Id, month, "gift")
		}
	}
	if cfg.EnableClaim {
		antiCheatToken := c.root.Cfg.Accounts.AntiCheatTokenFor(cookieFile)
		if c.isLocalSuccessCached(ctx, db, user.Account.Id, month, "claim") {
			c.cmd.Printf("[vip-member-gift] 跳过领取：%s已领过。\n", month)
		} else if antiCheatToken == "" {
			c.cmd.Printf("[vip-member-gift] 跳过领取：accounts.antiCheatTokens 中未配置 %s 的 token\n", cookieFile)
		} else if ok, err := c.runClaim(ctx, eapiCli, cloud, cfg, user.Account.Id, protocol, antiCheatToken); err != nil {
			c.cmd.Printf("[vip-member-gift] 领取失败: %v\n", err)
			if firstErr == nil {
				firstErr = err
			}
		} else if ok {
			c.saveLocalSuccessCache(ctx, db, user.Account.Id, month, "claim")
		}
	}
	return firstErr
}

func (c *VipMemberGift) validateCommonPrerequisites(cfg *config.VipMemberGiftConf) error {
	if cfg == nil {
		return fmt.Errorf("vipMemberGift configuration is not set in config.yaml")
	}
	if c.root.Cfg.Network == nil {
		return fmt.Errorf("network configuration is not set")
	}
	if strings.TrimSpace(cfg.Cloud.BaseURL) == "" {
		return fmt.Errorf("vipMemberGift.cloud.baseUrl is empty")
	}
	return nil
}

func (c *VipMemberGift) runGift(ctx context.Context, eapiCli *eapi.Api, weapiCli *weapi.Api, cloud *vipMemberGiftCloudClient, cfg *config.VipMemberGiftConf, uid int64, protocol vipMemberGiftProtocol) (bool, error) {
	var totalDays int64
	var availableDays int64
	var tokenExpireTime int64
	vipLevel := int64(0)

	if page, err := eapiCli.VipMemberGiftPageInfo(ctx, &eapi.VipMemberGiftPageInfoReq{VipMemberGiftBaseReq: vipMemberGiftBaseReq(protocol)}); err != nil {
		c.cmd.Printf("[vip-member-gift] page/info failed, will use detail fallback: %v\n", err)
	} else if page.Code == 200 {
		vipLevel = int64(page.Data.VipLevel)
		totalDays = int64(page.Data.SendVipDayTotal)
		availableDays = int64(page.Data.SendVipDayLeft)
		if page.Data.CanSendVip == 0 && availableDays <= 0 {
			c.cmd.Printf("[vip-member-gift] 跳过赠送：没有剩余可用赠送天数 (%s)\n", page.Data.Desc)
			return false, nil
		}
	}

	resp, err := eapiCli.VipMemberGiftTokenCreate(ctx, &eapi.VipMemberGiftTokenCreateReq{VipMemberGiftBaseReq: vipMemberGiftBaseReq(protocol)})
	if err != nil {
		return false, fmt.Errorf("token/create: %w", err)
	}
	if resp.Code != 200 || strings.TrimSpace(resp.Data.Token) == "" {
		return false, fmt.Errorf("token/create failed: code=%d msg=%s", resp.Code, resp.Message)
	}
	token := strings.TrimSpace(resp.Data.Token)
	//c.cmd.Printf("[vip-member-gift] created gift token: hash=%s\n", shortTokenHash(token))

	if detail, err := eapiCli.VipMemberGiftDetail(ctx, &eapi.VipMemberGiftDetailReq{
		VipMemberGiftBaseReq: vipMemberGiftBaseReq(protocol),
		Token:                token,
	}); err != nil {
		c.cmd.Printf("[vip-member-gift] detail after token/create failed, will use VIP level fallback: %v\n", err)
	} else if detail.Code == 200 {
		if totalDays <= 0 && detail.Data.InviterTotalDays != nil {
			totalDays = *detail.Data.InviterTotalDays
		}
		if tokenExpireTime <= 0 {
			tokenExpireTime = detail.Data.TokenExpireTime
		}
	}

	if totalDays <= 0 {
		if grow, err := weapiCli.VipGrowPoint(ctx, &weapi.VipGrowPointReq{}); err == nil && grow.Code == 200 {
			if vipLevel <= 0 {
				vipLevel = grow.Data.LevelCard.Level
			}
			if vipLevel == 0 {
				vipLevel = grow.Data.UserLevel.Level
			}
			totalDays = vipMemberGiftDaysFromLevel(vipLevel)
		}
	}
	if totalDays <= 0 {
		return false, fmt.Errorf("could not determine gift available days; skip publishing token")
	}
	if availableDays <= 0 {
		availableDays = totalDays
	}

	upsert, err := cloud.UpsertToken(ctx, map[string]interface{}{
		"token":            token,
		"donorUid":         strconv.FormatInt(uid, 10),
		"month":            currentVipMemberGiftMonth(),
		"vipLevel":         vipLevelString(vipLevel),
		"inviterTotalDays": totalDays,
		"availableDays":    availableDays,
		"tokenExpireTime":  tokenExpireTime,
		"source":           vipMemberGiftCloudSource,
	})
	if err != nil {
		return false, fmt.Errorf("cloud upsert token: %w", err)
	}
	if upsert.Token != nil {
		c.cmd.Printf("[vip-member-gift] 赠送VIP发布到云端：hash=%s，可用天数=%d\n", shortHash(upsert.TokenHash), upsert.Token.AvailableDays)
	} else {
		c.cmd.Printf("[vip-member-gift] 赠送VIP发布到云端：hash=%s\n", shortHash(upsert.TokenHash))
	}
	return true, nil
}

func (c *VipMemberGift) runClaim(ctx context.Context, eapiCli *eapi.Api, cloud *vipMemberGiftCloudClient, cfg *config.VipMemberGiftConf, uid int64, protocol vipMemberGiftProtocol, antiCheatToken string) (bool, error) {
	month := currentVipMemberGiftMonth()
	receiverUID := strconv.FormatInt(uid, 10)

	available, err := cloud.AvailableToken(ctx, month, receiverUID, receiverUID)
	if err != nil {
		return false, fmt.Errorf("cloud available token: %w", err)
	}
	if available.Claimed {
		c.cmd.Printf("[vip-member-gift] 跳过领取：%s已领过。\n", month)
		return true, nil
	}
	if available.Token == nil || strings.TrimSpace(available.Token.Token) == "" {
		c.cmd.Println("[vip-member-gift] 没有可领取的云端VIP。")
		return false, nil
	}
	token := strings.TrimSpace(available.Token.Token)
	//c.cmd.Printf("[vip-member-gift] selected cloud token: hash=%s availableDays=%d\n", shortHash(available.Token.TokenHash), available.Token.AvailableDays)

	detail, err := eapiCli.VipMemberGiftDetail(ctx, &eapi.VipMemberGiftDetailReq{
		VipMemberGiftBaseReq: vipMemberGiftBaseReq(protocol),
		Token:                token,
	})
	if err != nil {
		_ = cloud.TokenFail(ctx, month, receiverUID, available.Token.TokenHash, "failed", err.Error(), 0)
		return false, fmt.Errorf("detail/info/get: %w", err)
	}
	if detail.Code != 200 {
		_ = cloud.TokenFail(ctx, month, receiverUID, available.Token.TokenHash, "failed", detail.Message, 0)
		return false, fmt.Errorf("detail/info/get failed: code=%d msg=%s", detail.Code, detail.Message)
	}
	if detail.Data.TokenExpireTime > 0 && detail.Data.TokenExpireTime <= time.Now().UnixMilli() {
		_ = cloud.TokenFail(ctx, month, receiverUID, available.Token.TokenHash, "expired", "token expired", 0)
		c.cmd.Println("[vip-member-gift] 领取失败：云端VIP已过期")
		return false, nil
	}
	if detail.Data.CurrentUserRecordID != nil && *detail.Data.CurrentUserRecordID > 0 {
		duration := detail.Data.Duration
		if duration <= 0 {
			duration = 1
		}
		_, err := cloud.ClaimSuccess(ctx, map[string]interface{}{
			"month":       month,
			"receiverUid": receiverUID,
			"tokenHash":   available.Token.TokenHash,
			"recordId":    strconv.FormatInt(*detail.Data.CurrentUserRecordID, 10),
			"duration":    duration,
			"rewardName":  detail.Data.RewardName,
			"acceptedAt":  detail.Data.AcceptTime,
		})
		if err != nil {
			return false, fmt.Errorf("cloud claim success for existing record: %w", err)
		}
		c.cmd.Printf("[vip-member-gift] 跳过领取：%s已领过。\n", month)
		return true, nil
	}

	accept, err := eapiCli.VipMemberGiftAccept(ctx, &eapi.VipMemberGiftAcceptReq{
		VipMemberGiftBaseReq: vipMemberGiftBaseReq(protocol),
		Token:                token,
		Refer:                strings.TrimSpace(cfg.Refer),
		CheckToken:           antiCheatToken,
	})
	if err != nil {
		_ = cloud.TokenFail(ctx, month, receiverUID, available.Token.TokenHash, "failed", err.Error(), 0)
		return false, fmt.Errorf("accept: %w", err)
	}
	if accept.Code != 200 {
		reason := "failed"
		if strings.Contains(accept.Message, "过期") {
			reason = "expired"
		}
		_ = cloud.TokenFail(ctx, month, receiverUID, available.Token.TokenHash, reason, accept.Message, 0)
		return false, fmt.Errorf("accept failed: code=%d msg=%s", accept.Code, accept.Message)
	}
	if accept.Data.Duration <= 0 {
		return false, fmt.Errorf("accept succeeded but duration is empty")
	}

	claim, err := cloud.ClaimSuccess(ctx, map[string]interface{}{
		"month":       month,
		"receiverUid": receiverUID,
		"tokenHash":   available.Token.TokenHash,
		"recordId":    strconv.FormatInt(accept.Data.RecordID, 10),
		"duration":    accept.Data.Duration,
		"rewardName":  accept.Data.RewardName,
		"acceptedAt":  time.Now().UnixMilli(),
	})
	if err != nil {
		return false, fmt.Errorf("cloud claim success: %w", err)
	}
	c.printClaimResult(claim)
	return true, nil
}

func (c *VipMemberGift) printClaimResult(data *vipMemberGiftCloudClaimSuccessData) {
	if data == nil || data.Claim == nil {
		return
	}
	reward := data.Claim.RewardName
	if reward == "" {
		reward = fmt.Sprintf("%d天VIP", data.Claim.Duration)
	}
	c.cmd.Printf("[vip-member-gift] 领取云端VIP成功：%s。\n", reward)
}

func newVipMemberGiftCloudClient(baseURL, token string) *vipMemberGiftCloudClient {
	return &vipMemberGiftCloudClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *vipMemberGiftCloudClient) UpsertToken(ctx context.Context, body map[string]interface{}) (*vipMemberGiftCloudUpsertData, error) {
	var reply vipMemberGiftCloudUpsertData
	if err := c.post(ctx, "/tokens/upsert", body, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

func (c *vipMemberGiftCloudClient) AvailableToken(ctx context.Context, month, receiverUID, excludeDonorUID string) (*vipMemberGiftCloudAvailableData, error) {
	query := url.Values{}
	query.Set("month", month)
	query.Set("receiverUid", receiverUID)
	if strings.TrimSpace(excludeDonorUID) != "" {
		query.Set("excludeDonorUid", strings.TrimSpace(excludeDonorUID))
	}
	var reply vipMemberGiftCloudAvailableData
	if err := c.get(ctx, "/tokens/available?"+query.Encode(), &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

func (c *vipMemberGiftCloudClient) ClaimStatus(ctx context.Context, month, receiverUID string) (*vipMemberGiftCloudStatusData, error) {
	query := url.Values{}
	query.Set("month", month)
	query.Set("receiverUid", receiverUID)
	var reply vipMemberGiftCloudStatusData
	if err := c.get(ctx, "/claims/status?"+query.Encode(), &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

func (c *vipMemberGiftCloudClient) ClaimSuccess(ctx context.Context, body map[string]interface{}) (*vipMemberGiftCloudClaimSuccessData, error) {
	var reply vipMemberGiftCloudClaimSuccessData
	if err := c.post(ctx, "/claims/success", body, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

func (c *vipMemberGiftCloudClient) TokenFail(ctx context.Context, month, receiverUID, tokenHash, reason, message string, availableDays int) error {
	body := map[string]interface{}{
		"month":       month,
		"receiverUid": receiverUID,
		"tokenHash":   tokenHash,
		"reason":      reason,
		"message":     message,
	}
	if availableDays > 0 {
		body["availableDays"] = availableDays
	}
	var reply map[string]interface{}
	return c.post(ctx, "/tokens/fail", body, &reply)
}

func (c *vipMemberGiftCloudClient) get(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

func (c *vipMemberGiftCloudClient) post(ctx context.Context, path string, body interface{}, out interface{}) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

func (c *vipMemberGiftCloudClient) do(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	if c.baseURL == "" {
		return fmt.Errorf("cloud baseURL is empty")
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("json.Marshal: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("NewRequestWithContext: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("X-Api-Key", c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cloud request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read cloud response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cloud status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var envelope vipMemberGiftCloudEnvelope[json.RawMessage]
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("decode cloud envelope: %w", err)
	}
	if !envelope.OK {
		if envelope.Error == "" {
			envelope.Error = "unknown cloud error"
		}
		return fmt.Errorf("%s", envelope.Error)
	}
	if out != nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return fmt.Errorf("decode cloud data: %w", err)
		}
	}
	return nil
}

func (c *VipMemberGift) detectProtocol(cli *api.Client, networkCfg *api.Config) (vipMemberGiftProtocol, error) {
	cookieOS := vipMemberGiftCookieValue(cli, "os")
	platform := vipMemberGiftPlatformFromText(cookieOS)
	if platform == "" && networkCfg != nil {
		platform = vipMemberGiftPlatformFromText(networkCfg.UserAgent.XEAPI)
	}
	if platform == "" && networkCfg != nil {
		platform = vipMemberGiftPlatformFromText(networkCfg.UserAgent.EAPI)
	}
	if platform == "" {
		platform = vipMemberGiftPlatformFromText(cli.UserAgent(api.CryptoModeEAPI))
	}

	switch platform {
	case "android":
		if strings.TrimSpace(cli.UserAgent(api.CryptoModeXEAPI)) == "" {
			return vipMemberGiftProtocol{}, fmt.Errorf("network.user_agent.xeapi is empty; Android vip-member-gift requires a mobile cookie and matching Android XEAPI UA")
		}
		return vipMemberGiftProtocol{Mode: api.CryptoModeXEAPI, Platform: "Android", CookieOS: cookieOS}, nil
	case "ios":
		return vipMemberGiftProtocol{Mode: api.CryptoModeEAPI, Platform: "iOS", CookieOS: cookieOS}, nil
	default:
		return vipMemberGiftProtocol{}, fmt.Errorf("could not detect mobile platform from cookie os or user_agent; vip-member-gift requires Android or iOS mobile session")
	}
}

func vipMemberGiftBaseReq(protocol vipMemberGiftProtocol) eapi.VipMemberGiftBaseReq {
	return eapi.VipMemberGiftBaseReq{CryptoMode: protocol.Mode}
}

func vipMemberGiftCookieValue(cli *api.Client, names ...string) string {
	for _, baseURL := range []string{"https://music.163.com", "https://interface3.music.163.com"} {
		for _, name := range names {
			if ck, ok := cli.Cookie(baseURL, name); ok && strings.TrimSpace(ck.Value) != "" {
				value, err := url.QueryUnescape(ck.Value)
				if err != nil {
					value = ck.Value
				}
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
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

func currentVipMemberGiftMonth() string {
	return time.Now().Format("2006-01")
}

func vipMemberGiftCacheKey(uid int64, month, action string) string {
	return fmt.Sprintf("vip-member-gift:%d:%s:%s", uid, month, action)
}

func vipMemberGiftCacheTTL() time.Duration {
	return 45 * 24 * time.Hour
}

func (c *VipMemberGift) isLocalSuccessCached(ctx context.Context, db database.Database, uid int64, month, action string) bool {
	if db == nil {
		return false
	}
	exists, err := db.Exists(ctx, vipMemberGiftCacheKey(uid, month, action))
	if err != nil {
		c.cmd.Printf("[vip-member-gift] local cache check failed (%s): %v\n", action, err)
		return false
	}
	return exists
}

func (c *VipMemberGift) saveLocalSuccessCache(ctx context.Context, db database.Database, uid int64, month, action string) {
	if db == nil {
		return
	}
	if err := db.Set(ctx, vipMemberGiftCacheKey(uid, month, action), "1", vipMemberGiftCacheTTL()); err != nil {
		c.cmd.Printf("[vip-member-gift] local cache save failed (%s): %v\n", action, err)
	}
}

func shortTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return shortHash(hex.EncodeToString(sum[:]))
}

func shortHash(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func vipMemberGiftDaysFromLevel(level int64) int64 {
	switch level {
	case 2, 3, 4:
		return 98
	case 5, 6:
		return 128
	case 7:
		return 258
	default:
		return 0
	}
}

func vipLevelString(level int64) string {
	if level <= 0 {
		return ""
	}
	return fmt.Sprintf("V%d", level)
}
