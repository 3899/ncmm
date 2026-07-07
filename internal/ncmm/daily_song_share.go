// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package ncmm

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/3899/ncmm/api"
	"github.com/3899/ncmm/api/eapi"
	"github.com/3899/ncmm/api/types"
	"github.com/3899/ncmm/api/weapi"
	"github.com/3899/ncmm/config"
	"github.com/3899/ncmm/pkg/log"

	"github.com/spf13/cobra"
)

const (
	defaultDailySongSharePlaylistId  = "13848930701"
	maxDailySongShareLotteryAttempts = 10
)

type DailySongShareOpts struct {
	CookieFile string
}

type DailySongShare struct {
	root *Root
	cmd  *cobra.Command
	l    *log.Logger
	rng  *rand.Rand
	opts DailySongShareOpts
}

type dailySongShareSong struct {
	Id        int64
	Name      string
	Artists   []string
	AlbumName string
	CoverURL  string
}

func NewDailySongShare(root *Root, l *log.Logger) *DailySongShare {
	c := &DailySongShare{
		root: root,
		l:    l,
		rng:  rand.New(rand.NewSource(time.Now().UnixNano())),
		cmd: &cobra.Command{
			Use:     "daily-song-share",
			Short:   "[need login] Share a random song from the daily recommendation playlist",
			Example: "  ncmm daily-song-share --cookie-file run/cookie.json",
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context())
	}
	return c
}

func (c *DailySongShare) Command() *cobra.Command {
	return c.cmd
}

func (c *DailySongShare) addFlags() {
	c.cmd.Flags().StringVar(&c.opts.CookieFile, "cookie-file", "", "cookie file path")
}

func (c *DailySongShare) getConfig() (*config.DailySongShareConf, error) {
	if c.root.Cfg.DailySongShare != nil {
		return c.root.Cfg.DailySongShare, nil
	}
	return nil, fmt.Errorf("dailySongShare configuration is not set in config.yaml")
}

func (c *DailySongShare) execute(ctx context.Context) error {
	cfg, err := c.getConfig()
	if err != nil {
		return err
	}
	if err := c.validatePrerequisites(cfg); err != nil {
		c.cmd.Printf("[daily-song-share] skipped: %v\n", err)
		return nil
	}

	var queue []string
	if c.opts.CookieFile != "" {
		queue = append(queue, c.opts.CookieFile)
	} else {
		if c.root.Cfg.Accounts == nil {
			return fmt.Errorf("missing accounts section")
		}
		if cfg.EnableMain && c.root.Cfg.Accounts.Main != "" {
			queue = append(queue, c.root.Cfg.Accounts.Main)
		}
	}

	if len(queue) == 0 {
		c.cmd.Println("[daily-song-share] no available accounts, please check config.yaml")
		return nil
	}

	for i, cookieFile := range queue {
		c.cmd.Printf("[daily-song-share] start account (%s)\n", cookieFile)
		if _, err := c.ExecuteForCookie(ctx, cookieFile); err != nil {
			c.cmd.Printf("[daily-song-share] account failed (%s): %v\n", cookieFile, err)
		}
		c.cmd.Println("[daily-song-share] --------------------------------------------------")
		if i < len(queue)-1 {
			c.sleepBetweenAccounts(ctx, cookieFile)
		}
	}
	return nil
}

func (c *DailySongShare) sleepBetweenAccounts(ctx context.Context, currentAccount string) {
	sleepSec := 5 + c.rng.Intn(16)
	c.cmd.Printf("[daily-song-share] account (%s) done, wait %d seconds before next account...\n", currentAccount, sleepSec)
	select {
	case <-ctx.Done():
	case <-time.After(time.Duration(sleepSec) * time.Second):
	}
}

func (c *DailySongShare) ExecuteForCookie(ctx context.Context, cookieFile string) (int64, error) {
	cfg, err := c.getConfig()
	if err != nil {
		return 0, err
	}
	if err := c.validatePrerequisites(cfg); err != nil {
		return 0, err
	}

	networkCfg := c.root.Cfg.Network
	absPath, err := filepath.Abs(cookieFile)
	if err != nil {
		return 0, fmt.Errorf("resolve cookie path failed: %w", err)
	}

	cfgCopy := *networkCfg
	cfgCopy.Cookie.Filepath = absPath
	cli, err := api.NewClient(&cfgCopy, c.l)
	if err != nil {
		return 0, fmt.Errorf("NewClient: %w", err)
	}
	defer cli.Close(ctx)

	eapiCli := eapi.New(cli)
	weapiCli := weapi.New(cli)

	user, err := weapiCli.GetUserInfo(ctx, &weapi.GetUserInfoReq{})
	if err != nil {
		return 0, fmt.Errorf("verify login failed: %w", err)
	}
	if user.Code != 200 || user.Profile == nil || user.Account == nil {
		return 0, fmt.Errorf("user not logged in or session expired")
	}
	syncSessionConfig(ctx, cli, cookieFile, user.Account.Id, nil, c.root.Cfg.Database)
	c.cmd.Printf("[daily-song-share] current account: uid=%v nickname=%q\n", user.Account.Id, user.Profile.Nickname)

	song, playlistCover, err := c.selectSong(ctx, weapiCli, cfg)
	if err != nil {
		return 0, err
	}
	c.cmd.Printf("[daily-song-share] selected song: %s - %s (%d)\n", song.ArtistText(), song.Name, song.Id)

	pics, imageSource, err := c.uploadShareImage(ctx, eapiCli, cfg, song, playlistCover)
	if err != nil {
		return 0, err
	}
	if imageSource != "" {
		c.cmd.Printf("[daily-song-share] selected image: %s\n", imageSource)
	}

	title := c.buildTitle(ctx, eapiCli, cfg, song)
	msg := c.buildMessage(cfg, song)
	activityInfoList := c.buildActivityInfoList(cfg)

	c.cmd.Printf("[daily-song-share] publishing: title=%q song=%d\n", title, song.Id)
	resp, err := eapiCli.DailySongSharePublish(ctx, &eapi.DailySongSharePublishReq{
		Id:               strconv.FormatInt(song.Id, 10),
		UID:              strconv.FormatInt(user.Account.Id, 10),
		Title:            title,
		Msg:              msg,
		Type:             "song",
		Pics:             pics,
		ActivityInfoList: activityInfoList,
		CheckToken:       strings.TrimSpace(cfg.AntiCheatToken),
		Header:           "{}",
	})
	if err != nil {
		return 0, fmt.Errorf("publish daily song share failed: %w", err)
	}
	if resp.Code != 200 {
		return 0, fmt.Errorf("publish daily song share failed: code=%d", resp.Code)
	}
	c.cmd.Printf("[daily-song-share] publish success, eventId=%d\n", resp.Id)

	if cfg.Lottery != nil && cfg.Lottery.Enabled {
		c.runLottery(ctx, eapiCli, cfg)
	}

	if cfg.AutoDelete != nil && *cfg.AutoDelete {
		delay := 5 + c.rng.Intn(26)
		c.cmd.Printf("[daily-song-share] auto delete enabled, wait %d seconds...\n", delay)
		select {
		case <-ctx.Done():
			return resp.Id, ctx.Err()
		case <-time.After(time.Duration(delay) * time.Second):
		}
		delResp, err := eapiCli.EventDelete(ctx, &eapi.EventDeleteReq{Id: resp.Id})
		if err != nil {
			c.cmd.Printf("[daily-song-share] auto delete failed: %v\n", err)
		} else if delResp.Code != 200 {
			c.cmd.Printf("[daily-song-share] auto delete failed: code=%d\n", delResp.Code)
		} else {
			c.cmd.Printf("[daily-song-share] auto delete success, eventId=%d\n", resp.Id)
		}
	}

	return resp.Id, nil
}

func (c *DailySongShare) validatePrerequisites(cfg *config.DailySongShareConf) error {
	if cfg == nil {
		return fmt.Errorf("dailySongShare configuration is not set in config.yaml")
	}
	if strings.TrimSpace(cfg.AntiCheatToken) == "" {
		return fmt.Errorf("dailySongShare.antiCheatToken is empty; daily song share requires a mobile cookie, matching mobile UA, and X-antiCheatToken captured from the same mobile session")
	}
	if c.root.Cfg.Network == nil || strings.TrimSpace(c.root.Cfg.Network.UserAgent.XEAPI) == "" {
		return fmt.Errorf("network.user_agent.xeapi is empty; daily song share requires a mobile cookie, matching mobile UA, and X-antiCheatToken captured from the same mobile session")
	}
	return nil
}

func (c *DailySongShare) selectSong(ctx context.Context, weapiCli *weapi.Api, cfg *config.DailySongShareConf) (dailySongShareSong, string, error) {
	if songId := strings.TrimSpace(cfg.SongId); songId != "" {
		song, err := c.fetchSongDetail(ctx, weapiCli, songId)
		if err != nil {
			return dailySongShareSong{}, "", err
		}
		return song, "", nil
	}

	playlistId := strings.TrimSpace(cfg.PlaylistId)
	if playlistId == "" {
		playlistId = defaultDailySongSharePlaylistId
	}
	playlist, err := weapiCli.PlaylistDetail(ctx, &weapi.PlaylistDetailReq{Id: playlistId, N: "300", S: "5"})
	if err != nil {
		return dailySongShareSong{}, "", fmt.Errorf("fetch playlist detail failed: %w", err)
	}
	if playlist.Code != 200 {
		return dailySongShareSong{}, "", fmt.Errorf("fetch playlist detail failed: code=%d", playlist.Code)
	}
	song, err := c.pickSong(ctx, weapiCli, playlist)
	if err != nil {
		return dailySongShareSong{}, "", err
	}
	return song, playlist.Playlist.CoverImgUrl, nil
}

func (c *DailySongShare) pickSong(ctx context.Context, weapiCli *weapi.Api, playlist *weapi.PlaylistDetailResp) (dailySongShareSong, error) {
	var ids []int64
	for _, item := range playlist.Playlist.TrackIds {
		if item.Id > 0 {
			ids = append(ids, item.Id)
		}
	}
	if len(ids) == 0 {
		for _, item := range playlist.Playlist.Tracks {
			if item.Id > 0 {
				ids = append(ids, item.Id)
			}
		}
	}
	ids = uniqueInt64s(ids)
	if len(ids) == 0 {
		return dailySongShareSong{}, fmt.Errorf("playlist has no available songs")
	}

	return c.fetchSongDetail(ctx, weapiCli, strconv.FormatInt(ids[c.rng.Intn(len(ids))], 10))
}

func (c *DailySongShare) fetchSongDetail(ctx context.Context, weapiCli *weapi.Api, id string) (dailySongShareSong, error) {
	detail, err := weapiCli.SongDetail(ctx, &weapi.SongDetailReq{
		C: []weapi.SongDetailReqList{{Id: strings.TrimSpace(id), V: 0}},
	})
	if err != nil {
		return dailySongShareSong{}, fmt.Errorf("fetch selected song detail failed: %w", err)
	}
	if detail.Code != 200 || len(detail.Songs) == 0 {
		return dailySongShareSong{}, fmt.Errorf("fetch selected song detail failed: code=%d songs=%d", detail.Code, len(detail.Songs))
	}

	song := detail.Songs[0]
	return dailySongShareSong{
		Id:        song.Id,
		Name:      song.Name,
		Artists:   artistNames(song.Ar),
		AlbumName: song.Al.Name,
		CoverURL:  song.Al.PicUrl,
	}, nil
}

func artistNames(artists []types.Artist) []string {
	names := make([]string, 0, len(artists))
	for _, artist := range artists {
		if strings.TrimSpace(artist.Name) != "" {
			names = append(names, strings.TrimSpace(artist.Name))
		}
	}
	return names
}

func (s dailySongShareSong) ArtistText() string {
	if len(s.Artists) == 0 {
		return "Unknown Artist"
	}
	return strings.Join(s.Artists, "/")
}

func (s dailySongShareSong) Link() string {
	return fmt.Sprintf("https://music.163.com/song?id=%d", s.Id)
}

func (c *DailySongShare) uploadShareImage(ctx context.Context, eapiCli *eapi.Api, cfg *config.DailySongShareConf, song dailySongShareSong, playlistCover string) (string, string, error) {
	candidates := c.imageCandidates(ctx, eapiCli, cfg, song, playlistCover)
	if len(candidates) == 0 {
		return "", "", fmt.Errorf("no image candidate available for daily song share")
	}

	for _, imageURL := range shuffleSlice(candidates) {
		tmpFile, err := downloadImageToTemp(ctx, imageURL)
		if err != nil {
			c.cmd.Printf("[daily-song-share] image download failed (%s): %v\n", imageURL, err)
			continue
		}
		removeTemp := tmpFile != imageURL
		pics, err := eapiCli.EventUploadImage(ctx, tmpFile)
		if removeTemp {
			_ = os.Remove(tmpFile)
		}
		if err != nil {
			c.cmd.Printf("[daily-song-share] image upload failed (%s): %v\n", imageURL, err)
			continue
		}
		return pics, imageURL, nil
	}
	return "", "", fmt.Errorf("all image candidates failed")
}

func (c *DailySongShare) imageCandidates(ctx context.Context, eapiCli *eapi.Api, cfg *config.DailySongShareConf, song dailySongShareSong, playlistCover string) []string {
	custom := c.resolveConfiguredImageURLs(ctx, cfg)
	mode := strings.ToLower(strings.TrimSpace(cfg.ImageMode))
	if mode == "" {
		mode = "songcover"
	}

	var candidates []string
	appendURL := func(url string) {
		if strings.TrimSpace(url) != "" {
			candidates = append(candidates, strings.TrimSpace(url))
		}
	}

	switch mode {
	case "custom":
		candidates = append(candidates, custom...)
		appendURL(song.CoverURL)
		appendURL(playlistCover)
	case "playlistcover", "playlist-cover":
		appendURL(playlistCover)
		candidates = append(candidates, custom...)
		appendURL(song.CoverURL)
	default:
		appendURL(song.CoverURL)
		candidates = append(candidates, custom...)
		appendURL(playlistCover)
	}

	return uniqueStrings(candidates)
}

func (c *DailySongShare) resolveConfiguredImageURLs(ctx context.Context, cfg *config.DailySongShareConf) []string {
	sources := []string(cfg.ImageURLs)
	if len(uniqueStrings(sources)) == 0 && c.root.Cfg.Note != nil {
		sources = []string(c.root.Cfg.Note.ImageURLs)
	}
	if len(uniqueStrings(sources)) == 0 {
		return nil
	}
	n := NewNote(c.root, c.l)
	return uniqueStrings(n.resolveImageURLs(ctx, sources))
}

func (c *DailySongShare) buildTitle(ctx context.Context, eapiCli *eapi.Api, cfg *config.DailySongShareConf, song dailySongShareSong) string {
	mode := strings.ToLower(strings.TrimSpace(cfg.TitleMode))
	if mode == "song" {
		return truncateRunes(fmt.Sprintf("今日推荐：%s", song.Name), 60)
	}

	title := c.pickString(c.dailyTitles(cfg))
	if title == "" {
		title = "今日音乐分享"
	}
	return truncateRunes(applyDailySongTemplate(title, song), 60)
}

func (c *DailySongShare) buildMessage(cfg *config.DailySongShareConf, song dailySongShareSong) string {
	msg := c.pickString(c.dailyMessages(cfg))
	if msg == "" {
		msg = "分享一首今天听到的好歌。"
	}
	return strings.TrimSpace(applyDailySongTemplate(msg, song))
}

func (c *DailySongShare) runLottery(ctx context.Context, eapiCli *eapi.Api, cfg *config.DailySongShareConf) {
	lotteryCfg := cfg.Lottery
	if lotteryCfg == nil || !lotteryCfg.Enabled {
		return
	}

	autoRegister := true
	if lotteryCfg.AutoRegister != nil {
		autoRegister = *lotteryCfg.AutoRegister
	}
	if autoRegister {
		resp, err := eapiCli.DailySongShareRegister(ctx, &eapi.DailySongShareRegisterReq{})
		if err != nil {
			c.cmd.Printf("[daily-song-share] warn: daily song registration failed: %v\n", err)
		} else if resp.Code != 200 {
			c.cmd.Printf("[daily-song-share] warn: daily song registration failed: code=%d\n", resp.Code)
		}
	}

	activityId := parseDailySongLotteryActivityId(lotteryCfg.ActivityId)
	lotteryAttempts := 1
	guide, err := eapiCli.DailySongShareRegistrationGuide(ctx, &eapi.DailySongShareRegistrationGuideReq{})
	if err != nil {
		c.cmd.Printf("[daily-song-share] warn: fetch lottery guide failed: %v\n", err)
	} else if guide.Code != 200 {
		c.cmd.Printf("[daily-song-share] warn: fetch lottery guide failed: code=%d\n", guide.Code)
	} else {
		if guide.Data.ActivityInterestId > 0 {
			activityId = guide.Data.ActivityInterestId
		}
		g := guide.Data.RegisteredGuide
		c.cmd.Printf("[daily-song-share] lottery guide: %s, reward=%d, used=%d, alreadyPub=%v\n", g.SignTip, g.RewardCount, g.HaveRewardCount, g.AlreadyPubEvent)
		if g.SignTip != "" && g.RewardCount <= 0 {
			c.cmd.Println("[daily-song-share] no lottery chance available; skipped")
			return
		}
		remaining := dailySongLotteryRemainingAttempts(g.RewardCount, g.HaveRewardCount)
		if g.RewardCount > 0 && remaining <= 0 {
			c.cmd.Println("[daily-song-share] lottery chance already used; skipped")
			return
		}
		if remaining > 0 {
			lotteryAttempts = remaining
		}
	}
	if activityId <= 0 {
		c.cmd.Println("[daily-song-share] lottery activityId is empty; skipped")
		return
	}

	lotteryAttempts = clampDailySongLotteryAttempts(lotteryAttempts)
	c.cmd.Printf("[daily-song-share] starting lottery: activityId=%d attempts=%d\n", activityId, lotteryAttempts)
	for i := 0; i < lotteryAttempts; i++ {
		lottery, err := eapiCli.DailySongShareLottery(ctx, &eapi.DailySongShareLotteryReq{
			ActivityId: activityId,
			CheckToken: strings.TrimSpace(cfg.AntiCheatToken),
		})
		if err != nil {
			c.cmd.Printf("[daily-song-share] lottery failed [%d/%d]: %v\n", i+1, lotteryAttempts, err)
			return
		}
		if lottery.Code != 200 {
			c.cmd.Printf("[daily-song-share] lottery failed [%d/%d]: code=%d message=%s\n", i+1, lotteryAttempts, lottery.Code, lottery.Message)
			return
		}
		names := dailySongLotteryPrizeNames(lottery.Data.PrizeDetailInfoMap)
		if len(names) == 0 {
			c.cmd.Printf("[daily-song-share] lottery done [%d/%d], restChance=%d\n", i+1, lotteryAttempts, lottery.Data.RestChance)
		} else {
			c.cmd.Printf("[daily-song-share] lottery done [%d/%d]: %s, restChance=%d\n", i+1, lotteryAttempts, strings.Join(names, " / "), lottery.Data.RestChance)
		}
	}
}

func parseDailySongLotteryActivityId(value string) int64 {
	id, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return id
}

func dailySongLotteryRemainingAttempts(rewardCount, haveRewardCount int) int {
	remaining := rewardCount - haveRewardCount
	if remaining < 0 {
		return 0
	}
	return remaining
}

func clampDailySongLotteryAttempts(attempts int) int {
	if attempts <= 0 {
		return 1
	}
	if attempts > maxDailySongShareLotteryAttempts {
		return maxDailySongShareLotteryAttempts
	}
	return attempts
}

func dailySongLotteryPrizeNames(prizes map[string]eapi.DailySongShareLotteryPrizeDetail) []string {
	var names []string
	for _, prize := range prizes {
		if strings.TrimSpace(prize.PrizeName) != "" {
			names = append(names, strings.TrimSpace(prize.PrizeName))
			continue
		}
		if strings.TrimSpace(prize.WinPrizeDesc) != "" {
			names = append(names, strings.TrimSpace(prize.WinPrizeDesc))
		}
	}
	return uniqueStrings(names)
}

func (c *DailySongShare) dailyTitles(cfg *config.DailySongShareConf) []string {
	titles := c.collectTextPool("dailySongShare.titlesFile", cfg.Titles, cfg.TitlesFile)
	if len(titles) == 0 && c.root.Cfg.Note != nil {
		titles = c.collectTextPool("note.titlesFile", c.root.Cfg.Note.Titles, c.root.Cfg.Note.TitlesFile)
	}
	return uniqueStrings(titles)
}

func (c *DailySongShare) dailyMessages(cfg *config.DailySongShareConf) []string {
	messages := c.collectTextPool("dailySongShare.messagesFile", cfg.Messages, cfg.MessagesFile)
	if len(messages) == 0 && c.root.Cfg.Note != nil {
		messages = c.collectTextPool("note.messagesFile", c.root.Cfg.Note.Messages, c.root.Cfg.Note.MessagesFile)
	}
	return uniqueStrings(messages)
}

func (c *DailySongShare) collectTextPool(label string, inline []string, files config.StringOrSlice) []string {
	values := append([]string{}, inline...)
	for _, file := range uniqueStrings([]string(files)) {
		fileValues, err := parseMessagesFromFile(file)
		if err != nil {
			c.cmd.Printf("[daily-song-share] warn: read %s (%s) failed: %v\n", label, file, err)
			continue
		}
		values = append(values, fileValues...)
	}
	return values
}

func (c *DailySongShare) pickString(values []string) string {
	values = uniqueStrings(values)
	if len(values) == 0 {
		return ""
	}
	return values[c.rng.Intn(len(values))]
}

func applyDailySongTemplate(text string, song dailySongShareSong) string {
	replacer := strings.NewReplacer(
		"{song}", song.Name,
		"{artist}", song.ArtistText(),
		"{album}", song.AlbumName,
		"{link}", song.Link(),
		"{id}", strconv.FormatInt(song.Id, 10),
	)
	return replacer.Replace(text)
}

func truncateRunes(text string, max int) string {
	if max <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max])
}

func (c *DailySongShare) buildActivityInfoList(cfg *config.DailySongShareConf) string {
	topics := c.resolveActivityTopics(c.dailyTopics(cfg))
	var items []map[string]interface{}
	for _, topic := range topics {
		if strings.TrimSpace(topic.Id) == "" {
			continue
		}
		topicType := topic.Type
		if topicType == 0 {
			topicType = 3
		}
		selected := true
		if topic.Selected != nil {
			selected = *topic.Selected
		}
		canChange := true
		if topic.CanChange != nil {
			canChange = *topic.CanChange
		}
		items = append(items, map[string]interface{}{
			"id":        topic.Id,
			"type":      topicType,
			"subType":   topic.SubType,
			"name":      topic.Name,
			"desc":      nil,
			"selected":  selected,
			"canChange": canChange,
		})
	}
	if len(items) == 0 {
		return ""
	}
	data, err := json.Marshal(items)
	if err != nil {
		c.cmd.Printf("[daily-song-share] warn: marshal activityInfoList failed: %v\n", err)
		return ""
	}
	return string(data)
}

func (c *DailySongShare) dailyTopics(cfg *config.DailySongShareConf) []config.DailySongShareTopicConf {
	if len(cfg.Topics) > 0 {
		return cfg.Topics
	}
	selected := true
	canChange := true
	return []config.DailySongShareTopicConf{
		{Id: "13827903", Name: "音乐合伙人的乐迷团", Type: 3, SubType: 11, Selected: &selected, CanChange: &canChange},
		{Id: "195425749", Name: "申请音乐合伙人", Type: 2, Selected: &selected, CanChange: &canChange},
		{Id: "200773579", Name: "音乐合伙人星探计划", Type: 2, Selected: &selected, CanChange: &canChange},
	}
}

func (c *DailySongShare) resolveActivityTopics(topics []config.DailySongShareTopicConf) []config.DailySongShareTopicConf {
	resolved := make([]config.DailySongShareTopicConf, 0, len(topics))
	for _, topic := range topics {
		if strings.TrimSpace(topic.Id) == "" && strings.Contains(topic.Name, "乐迷团") {
			topic.Id = "13827903"
			if topic.Type == 0 {
				topic.Type = 3
			}
			if topic.SubType == 0 {
				topic.SubType = 11
			}
		}
		resolved = append(resolved, topic)
	}
	return resolved
}
