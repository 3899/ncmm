package eapi

import (
	"encoding/json"
	"testing"
)

func TestMusicianSongStatisticsResponses(t *testing.T) {
	var daily MusicianSongDailyResp
	if err := json.Unmarshal([]byte(`{"code":200,"message":"成功","data":{"indexs":{"play":265},"level":0}}`), &daily); err != nil {
		t.Fatal(err)
	}
	if daily.Code != 200 || daily.Data.Indexes.Play != 265 {
		t.Fatalf("unexpected daily response: %+v", daily)
	}

	var trend MusicianSongTrendResp
	if err := json.Unmarshal([]byte(`{"code":200,"message":"success","data":[{"dateTime":"2026-08-26","dataValue":"90"},{"dateTime":"2026-08-31","dataValue":"120"}]}`), &trend); err != nil {
		t.Fatal(err)
	}
	if trend.Code != 200 || len(trend.Data) != 2 || trend.Data[0].DateTime != "2026-08-26" || trend.Data[1].Value != "120" {
		t.Fatalf("unexpected trend response: %+v", trend)
	}
}

func TestMusicianSongTrendRequiresDateRange(t *testing.T) {
	api := &Api{}
	if _, err := api.MusicianSongTrend(t.Context(), nil); err == nil {
		t.Fatal("expected nil request to fail")
	}
	if _, err := api.MusicianSongTrend(t.Context(), &MusicianSongTrendReq{}); err == nil {
		t.Fatal("expected empty date range to fail")
	}
}
