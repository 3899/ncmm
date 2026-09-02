// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package ncmm

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/3899/ncmm/api/eapi"
	"github.com/3899/ncmm/internal/playstats"
	"github.com/3899/ncmm/pkg/database"
)

type musicianPlayStatsFetcher interface {
	MusicianSongDaily(context.Context, *eapi.MusicianSongDailyReq) (*eapi.MusicianSongDailyResp, error)
	MusicianSongTrend(context.Context, *eapi.MusicianSongTrendReq) (*eapi.MusicianSongTrendResp, error)
}

type musicianPlayStatsSync struct {
	Series         []playstats.DailyPoint
	BeforeReady    bool
	DailyRequested bool
	TrendRequested bool
}

func syncMusicianPlayStats(
	ctx context.Context,
	db database.Database,
	client musicianPlayStatsFetcher,
	account string,
	now time.Time,
) (musicianPlayStatsSync, error) {
	var result musicianPlayStatsSync
	if db == nil {
		return result, fmt.Errorf("database is unavailable")
	}
	if client == nil {
		return result, fmt.Errorf("statistics client is unavailable")
	}

	dates := playstats.RecentCompleteDates(now, 7)
	series, err := playstats.LoadSeries(ctx, db, account, dates)
	if err != nil {
		return result, err
	}
	result.Series = series
	chinaNow := now.In(playstats.ChinaLocation)
	if chinaNow.Hour() < playstats.OfficialDailyReadyHour {
		result.BeforeReady = true
		return result, nil
	}

	yesterday := dates[len(dates)-1]
	if !series[len(series)-1].Known {
		shouldRequest, markErr := playstats.MarkAttemptIfNew(ctx, db, account, "daily", yesterday)
		if markErr != nil {
			err = errors.Join(err, fmt.Errorf("记录昨日播放量请求: %w", markErr))
		} else if shouldRequest {
			result.DailyRequested = true
			response, requestErr := client.MusicianSongDaily(ctx, &eapi.MusicianSongDailyReq{})
			if requestErr != nil {
				err = errors.Join(err, fmt.Errorf("获取昨日有效播放量: %w", requestErr))
			} else if response.Code != 200 {
				err = errors.Join(err, fmt.Errorf("获取昨日有效播放量: code=%d message=%s", response.Code, response.Message))
			} else if saveErr := playstats.Save(ctx, db, account, playstats.Point{
				Date: yesterday, Count: response.Data.Indexes.Play,
				Source: "songs/daily", CollectedAt: chinaNow,
			}); saveErr != nil {
				err = errors.Join(err, fmt.Errorf("保存昨日有效播放量: %w", saveErr))
			}
		}
	}

	series, loadErr := playstats.LoadSeries(ctx, db, account, dates)
	if loadErr != nil {
		return result, errors.Join(err, loadErr)
	}
	result.Series = series
	historicalMissing := false
	for _, point := range series[:len(series)-1] {
		if !point.Known {
			historicalMissing = true
			break
		}
	}
	if historicalMissing {
		attemptDate := chinaNow.Format("2006-01-02")
		shouldRequest, markErr := playstats.MarkAttemptIfNew(ctx, db, account, "trend", attemptDate)
		if markErr != nil {
			err = errors.Join(err, fmt.Errorf("记录历史回补请求: %w", markErr))
		} else if shouldRequest {
			result.TrendRequested = true
			response, requestErr := client.MusicianSongTrend(ctx, &eapi.MusicianSongTrendReq{
				StartTime: dates[0], EndTime: yesterday, Type: 100,
			})
			if requestErr != nil {
				err = errors.Join(err, fmt.Errorf("回补有效播放历史: %w", requestErr))
			} else if response.Code != 200 {
				err = errors.Join(err, fmt.Errorf("回补有效播放历史: code=%d message=%s", response.Code, response.Message))
			} else {
				missing := make(map[string]bool)
				for _, point := range series[:len(series)-1] {
					missing[point.Date] = !point.Known
				}
				for _, point := range response.Data {
					if !missing[point.DateTime] {
						continue
					}
					count, parseErr := strconv.Atoi(point.Value)
					if parseErr != nil || count < 0 {
						err = errors.Join(err, fmt.Errorf("解析 %s 有效播放量 %q 失败", point.DateTime, point.Value))
						continue
					}
					if saveErr := playstats.Save(ctx, db, account, playstats.Point{
						Date: point.DateTime, Count: count,
						Source: "trend/get", CollectedAt: chinaNow,
					}); saveErr != nil {
						err = errors.Join(err, fmt.Errorf("保存 %s 有效播放量: %w", point.DateTime, saveErr))
					}
				}
			}
		}
	}

	result.Series, loadErr = playstats.LoadSeries(ctx, db, account, dates)
	return result, errors.Join(err, loadErr)
}

func (c *Musician) syncAndPrintPlayStats(ctx context.Context, mctx *musicianContext, cookieFile string) {
	if mctx == nil || mctx.db == nil {
		c.cmd.Println("    ⚠️ 本地数据库不可用，已跳过有效播放统计，未请求网易云")
		return
	}
	absPath, err := filepath.Abs(cookieFile)
	if err != nil {
		c.cmd.Printf("    ⚠️ 无法解析账号路径，已跳过有效播放统计: %v\n", err)
		return
	}
	account := playstats.CanonicalAccount(absPath, "")
	result, syncErr := syncMusicianPlayStats(ctx, mctx.db, mctx.eapiCli, account, time.Now())
	if result.BeforeReady {
		c.cmd.Println("    ℹ️ 官方昨日播放量将在 09:00 后更新，本次仅使用本地历史数据")
	}
	if syncErr != nil {
		c.cmd.Printf("    ⚠️ 有效播放统计同步不完整，已保留原有数据: %v\n", syncErr)
	}
	if len(result.Series) == 0 {
		return
	}
	known, total := 0, 0
	for _, point := range result.Series {
		if point.Known {
			known++
			total += point.Count
		}
	}
	latest := result.Series[len(result.Series)-1]
	latestText := "--"
	if latest.Known {
		latestText = strconv.Itoa(latest.Count)
	}
	requestText := "本地缓存"
	if result.DailyRequested || result.TrendRequested {
		requestText = "官方数据已同步"
	}
	c.cmd.Printf("    📈 有效播放统计: %s %s | 最近7日 %d 天合计 %d | %s\n", latest.Date, latestText, known, total, requestText)
}
