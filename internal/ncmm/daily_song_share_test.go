package ncmm

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/3899/ncmm/config"
)

func TestDailySongShareBuildMessageDoesNotAppendSongOrTopics(t *testing.T) {
	cfg := &config.DailySongShareConf{
		Messages: []string{"今天分享 {song}"},
		Topics: []config.DailySongShareTopicConf{
			{Name: "音乐合伙人的乐迷团", Id: "13827903", Type: 3, SubType: 11},
			{Name: "申请音乐合伙人", Id: "195425749", Type: 2},
		},
	}
	share := &DailySongShare{
		root: &Root{Cfg: &config.Config{DailySongShare: cfg}},
		rng:  rand.New(rand.NewSource(1)),
	}
	song := dailySongShareSong{
		Id:      3342899901,
		Name:    "Montagem Nada 2",
		Artists: []string{"Trispect", "makabaka"},
	}

	got := share.buildMessage(cfg, song)
	if got != "今天分享 Montagem Nada 2" {
		t.Fatalf("unexpected message: %q", got)
	}
	for _, forbidden := range []string{"今日推荐", song.Link(), "音乐合伙人的乐迷团", "#申请音乐合伙人"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("message should not contain %q: %q", forbidden, got)
		}
	}
}

func TestDailySongLotteryRemainingAttempts(t *testing.T) {
	tests := []struct {
		name   string
		reward int
		used   int
		want   int
	}{
		{name: "two chances unused", reward: 2, used: 0, want: 2},
		{name: "one chance used", reward: 2, used: 1, want: 1},
		{name: "all used", reward: 2, used: 2, want: 0},
		{name: "over used", reward: 2, used: 3, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dailySongLotteryRemainingAttempts(tt.reward, tt.used); got != tt.want {
				t.Fatalf("dailySongLotteryRemainingAttempts(%d, %d) = %d, want %d", tt.reward, tt.used, got, tt.want)
			}
		})
	}
}

func TestClampDailySongLotteryAttempts(t *testing.T) {
	if got := clampDailySongLotteryAttempts(0); got != 1 {
		t.Fatalf("zero attempts should fall back to one draw, got %d", got)
	}
	if got := clampDailySongLotteryAttempts(2); got != 2 {
		t.Fatalf("two attempts should be preserved, got %d", got)
	}
	if got := clampDailySongLotteryAttempts(maxDailySongShareLotteryAttempts + 1); got != maxDailySongShareLotteryAttempts {
		t.Fatalf("attempts should be capped at %d, got %d", maxDailySongShareLotteryAttempts, got)
	}
}
