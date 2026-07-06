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
