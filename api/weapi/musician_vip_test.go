package weapi

import (
	"encoding/json"
	"testing"
)

func TestMusicianVipInfoResp_Unmarshal(t *testing.T) {
	raw := `{
		"code": 200,
		"data": {
			"hasOpen": true,
			"isMusician": true,
			"canOpen": true,
			"furtherVipGetTime": 1789315200000,
			"hasFurtherTask": true,
			"taskStatus": true,
			"maintainDays": 824,
			"recentPlayCount30": 2753,
			"furtherTask": {
				"totalCompleteNum": 2,
				"progressRate": 2,
				"missionStatus": 100,
				"children": [
					{
						"name": "近30天有效播放达650次",
						"totalCompleteNum": 650,
						"progressRate": 650,
						"missionStatus": 100,
						"missionCode": "mission_code_recently_play_count"
					},
					{
						"name": "近30天内发布图文笔记4篇",
						"totalCompleteNum": 4,
						"progressRate": 4,
						"missionStatus": 100,
						"missionCode": "mission_code_musician_notebook_publish"
					}
				]
			},
			"unlockVipRight": true
		},
		"message": ""
	}`

	var resp MusicianVipInfoResp
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if resp.Code != 200 {
		t.Errorf("expected code 200, got %d", resp.Code)
	}
	if !resp.Data.CanOpen {
		t.Errorf("expected canOpen = true")
	}
	if !resp.Data.TaskStatus {
		t.Errorf("expected taskStatus = true")
	}
	if resp.Data.FurtherVipGetTime != 1789315200000 {
		t.Errorf("expected furtherVipGetTime 1789315200000, got %d", resp.Data.FurtherVipGetTime)
	}
	if resp.Data.FurtherTask == nil || len(resp.Data.FurtherTask.Children) != 2 {
		t.Fatalf("expected 2 children tasks")
	}
	if resp.Data.FurtherTask.Children[0].MissionCode != "mission_code_recently_play_count" {
		t.Errorf("unexpected missionCode: %s", resp.Data.FurtherTask.Children[0].MissionCode)
	}
}

func TestMusicianVipGetResp_Unmarshal(t *testing.T) {
	raw := `{"code":200,"data":true,"message":""}`
	var resp MusicianVipGetResp
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if resp.Code != 200 {
		t.Errorf("expected code 200, got %d", resp.Code)
	}
	if !resp.Data {
		t.Errorf("expected data = true")
	}
}
