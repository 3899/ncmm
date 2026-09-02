package ncmm

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/3899/ncmm/api/eapi"
	"github.com/3899/ncmm/api/types"
	"github.com/3899/ncmm/internal/playstats"
)

type playStatsMemoryDB struct {
	values map[string]string
}

func newPlayStatsMemoryDB() *playStatsMemoryDB {
	return &playStatsMemoryDB{values: make(map[string]string)}
}

func (db *playStatsMemoryDB) Get(_ context.Context, key string) (string, error) {
	value, ok := db.values[key]
	if !ok {
		return "", errors.New("not found")
	}
	return value, nil
}

func (db *playStatsMemoryDB) Set(_ context.Context, key, value string, _ ...time.Duration) error {
	db.values[key] = value
	return nil
}

func (db *playStatsMemoryDB) Exists(_ context.Context, key string) (bool, error) {
	_, ok := db.values[key]
	return ok, nil
}

func (db *playStatsMemoryDB) Increment(_ context.Context, key string, value int64, _ ...time.Duration) (int64, error) {
	old, _ := strconv.ParseInt(db.values[key], 10, 64)
	db.values[key] = strconv.FormatInt(old+value, 10)
	return old, nil
}

func (db *playStatsMemoryDB) Del(_ context.Context, key string) error {
	delete(db.values, key)
	return nil
}

func (db *playStatsMemoryDB) Close(context.Context) error { return nil }

type playStatsFetcherStub struct {
	dailyCalls int
	trendCalls int
	daily      int
	trend      []eapi.MusicianSongTrendPoint
	dailyErr   error
	trendErr   error
}

func (stub *playStatsFetcherStub) MusicianSongDaily(context.Context, *eapi.MusicianSongDailyReq) (*eapi.MusicianSongDailyResp, error) {
	stub.dailyCalls++
	if stub.dailyErr != nil {
		return nil, stub.dailyErr
	}
	return &eapi.MusicianSongDailyResp{RespCommon: types.RespCommon[eapi.MusicianSongDailyData]{
		Code: 200,
		Data: eapi.MusicianSongDailyData{Indexes: eapi.MusicianSongDailyIndexes{Play: stub.daily}},
	}}, nil
}

func (stub *playStatsFetcherStub) MusicianSongTrend(context.Context, *eapi.MusicianSongTrendReq) (*eapi.MusicianSongTrendResp, error) {
	stub.trendCalls++
	if stub.trendErr != nil {
		return nil, stub.trendErr
	}
	return &eapi.MusicianSongTrendResp{RespCommon: types.RespCommon[[]eapi.MusicianSongTrendPoint]{
		Code: 200, Data: stub.trend,
	}}, nil
}

func TestSyncMusicianPlayStatsColdStartThenUsesDatabase(t *testing.T) {
	db := newPlayStatsMemoryDB()
	fetcher := &playStatsFetcherStub{
		daily: 265,
		trend: []eapi.MusicianSongTrendPoint{
			{DateTime: "2026-08-26", Value: "90"},
			{DateTime: "2026-08-27", Value: "192"},
			{DateTime: "2026-08-28", Value: "100"},
			{DateTime: "2026-08-29", Value: "156"},
			{DateTime: "2026-08-30", Value: "147"},
			{DateTime: "2026-08-31", Value: "120"},
		},
	}
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, playstats.ChinaLocation)
	result, err := syncMusicianPlayStats(t.Context(), db, fetcher, "account", now)
	if err != nil {
		t.Fatal(err)
	}
	if fetcher.dailyCalls != 1 || fetcher.trendCalls != 1 || len(result.Series) != 7 {
		t.Fatalf("unexpected first sync: daily=%d trend=%d series=%+v", fetcher.dailyCalls, fetcher.trendCalls, result.Series)
	}
	total := 0
	for _, point := range result.Series {
		if !point.Known {
			t.Fatalf("point was not backfilled: %+v", point)
		}
		total += point.Count
	}
	if total != 1070 {
		t.Fatalf("total = %d, want 1070", total)
	}

	secondFetcher := &playStatsFetcherStub{}
	result, err = syncMusicianPlayStats(t.Context(), db, secondFetcher, "account", now)
	if err != nil {
		t.Fatal(err)
	}
	if secondFetcher.dailyCalls != 0 || secondFetcher.trendCalls != 0 || len(playstats.MissingDates(result.Series)) != 0 {
		t.Fatalf("cached sync made requests: daily=%d trend=%d series=%+v", secondFetcher.dailyCalls, secondFetcher.trendCalls, result.Series)
	}
}

func TestSyncMusicianPlayStatsBeforeNineMakesNoRequests(t *testing.T) {
	db := newPlayStatsMemoryDB()
	fetcher := &playStatsFetcherStub{daily: 265}
	now := time.Date(2026, 9, 2, 8, 59, 0, 0, playstats.ChinaLocation)
	result, err := syncMusicianPlayStats(t.Context(), db, fetcher, "account", now)
	if err != nil {
		t.Fatal(err)
	}
	if !result.BeforeReady || fetcher.dailyCalls != 0 || fetcher.trendCalls != 0 {
		t.Fatalf("unexpected pre-09:00 sync: %+v daily=%d trend=%d", result, fetcher.dailyCalls, fetcher.trendCalls)
	}
}

func TestSyncMusicianPlayStatsOnlyBackfillsHistoricalGap(t *testing.T) {
	db := newPlayStatsMemoryDB()
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, playstats.ChinaLocation)
	dates := playstats.RecentCompleteDates(now, 7)
	for _, date := range dates[1:] {
		if err := playstats.Save(t.Context(), db, "account", playstats.Point{Date: date, Count: 1}); err != nil {
			t.Fatal(err)
		}
	}
	fetcher := &playStatsFetcherStub{trend: []eapi.MusicianSongTrendPoint{{DateTime: dates[0], Value: "90"}}}
	result, err := syncMusicianPlayStats(t.Context(), db, fetcher, "account", now)
	if err != nil {
		t.Fatal(err)
	}
	if fetcher.dailyCalls != 0 || fetcher.trendCalls != 1 || len(playstats.MissingDates(result.Series)) != 0 {
		t.Fatalf("unexpected gap repair: daily=%d trend=%d series=%+v", fetcher.dailyCalls, fetcher.trendCalls, result.Series)
	}
}

func TestSyncMusicianPlayStatsDoesNotRetryFailuresOnSameDay(t *testing.T) {
	db := newPlayStatsMemoryDB()
	fetcher := &playStatsFetcherStub{
		dailyErr: errors.New("daily unavailable"),
		trendErr: errors.New("trend unavailable"),
	}
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, playstats.ChinaLocation)
	if _, err := syncMusicianPlayStats(t.Context(), db, fetcher, "account", now); err == nil {
		t.Fatal("expected upstream failures")
	}
	if _, err := syncMusicianPlayStats(t.Context(), db, fetcher, "account", now); err != nil {
		t.Fatalf("second sync should use attempt markers without requesting upstream: %v", err)
	}
	if fetcher.dailyCalls != 1 || fetcher.trendCalls != 1 {
		t.Fatalf("failed requests were repeated: daily=%d trend=%d", fetcher.dailyCalls, fetcher.trendCalls)
	}
}
