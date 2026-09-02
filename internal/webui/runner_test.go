package webui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunManagerHelperProcess(t *testing.T) {
	if len(os.Args) < 2 || os.Args[len(os.Args)-2] != "--run-manager-helper" {
		return
	}
	gate := os.Args[len(os.Args)-1]
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(gate); err == nil {
			os.Exit(0)
		}
		time.Sleep(10 * time.Millisecond)
	}
	os.Exit(2)
}

func TestParseRunRewards(t *testing.T) {
	logData := `[sign] >>>>>> 开始主账号签到 (cookie.json) <<<<<<
  [当前账号信息] Uid: 1311667400 | 昵称: 涂小惜 | 等级: 黑胶·贰 (Lv.2) | 头像: https://p1.music.126.net/main.jpg
  [账号身份] 音乐人
  --- 云贝任务 ---
  云贝今天已签到过
  🎉 成功领取云贝 [分享歌曲] 任务奖励，获得云贝: 100
  🎉 预约活动奖励领取成功，获得云贝数量：50
  👉 [执行后] 黑胶成长值最终状态: 今日已获得 42，当前成长值 946
  [云贝余额] 当前可用云贝: 1230
  [今日收益] 云贝 +2328 / 1230 | 成长值 12 / 3937
[musician vip] >>>>>> 开始主账号音乐人VIP进阶任务 (cookie.json) <<<<<<
  ✅ 已认证音乐人 | 维持天数: 30 天 | 近30天播放: 1200 次 | 解锁VIP权益: true
    - 当前进度: 35/100, 今日尚缺有效播放: 65 次
[sign] >>>>>> 开始辅助账号签到 (sub/2TH.json) <<<<<<
  [当前账号信息] Uid: 17609220462 | 昵称: 二号西柚 | 等级: 黑胶·壹 (Lv.1)
  --- 云贝任务 ---
  云贝签到成功
  暂无会员权益 (VIP 状态: 0)
  [今日收益] 云贝 +60 / 321 | 非 VIP 账号
[musician sign] >>>>>> 开始辅助账号音乐人日常签到 (sub/2TH.json) <<<<<<
  [musician sign] ❌ 辅助账号签到失败: 当前账号不是音乐人
`
	rewards := parseRunRewardsReader(strings.NewReader(logData))
	if len(rewards) != 2 {
		t.Fatalf("reward count = %d; want 2: %+v", len(rewards), rewards)
	}
	main := rewards[0]
	if main.Account != "cookie.json" || main.UID != "1311667400" || main.Nickname != "涂小惜" || main.AvatarURL != "https://p1.music.126.net/main.jpg" || main.Identity != "黑胶·贰 (Lv.2)" {
		t.Fatalf("unexpected main profile: %+v", main)
	}
	if !main.CookieKnown || !main.CookieValid || !main.MusicianKnown || !main.Musician || !main.YunbeiKnown || main.Yunbei != 2328 || !main.YunbeiCumulative || !main.YunbeiBalanceKnown || main.YunbeiBalance != 1230 || !main.GrowthKnown || main.GrowthToday != 12 || main.GrowthTotal != 3937 || !main.VIPKnown || !main.VIP || !main.EffectiveKnown || main.EffectivePlays != 35 || main.EffectiveTarget != 100 || !main.SignKnown || !main.Signed {
		t.Fatalf("unexpected main rewards: %+v", main)
	}
	secondary := rewards[1]
	if secondary.Account != "sub/2TH.json" || secondary.Yunbei != 60 || !secondary.YunbeiKnown || !secondary.YunbeiCumulative || secondary.YunbeiBalance != 321 || !secondary.YunbeiBalanceKnown || !secondary.VIPKnown || secondary.VIP || !secondary.MusicianKnown || secondary.Musician || secondary.GrowthKnown || !secondary.SignKnown || !secondary.Signed {
		t.Fatalf("unexpected secondary rewards: %+v", secondary)
	}
}

func TestParseRunRewardsTaskAccountMusicianIdentity(t *testing.T) {
	logData := `[task] >>> 账号 (one.json) 开始执行 [音乐人日常签到] 任务 <<<
  ✅ 已认证音乐人 (来自身份缓存)
[task] >>> 账号 (two.json) 开始执行 [音乐人VIP进阶] 任务 <<<
[task] ❌ 账号 (two.json) [音乐人VIP进阶] 执行失败: 当前账号不是音乐人
`
	rewards := parseRunRewardsReader(strings.NewReader(logData))
	if len(rewards) != 2 || !rewards[0].MusicianKnown || !rewards[0].Musician || !rewards[1].MusicianKnown || rewards[1].Musician {
		t.Fatalf("unexpected task musician identities: %+v", rewards)
	}
}

func TestParseRunRewardsLegacyYunbeiFallback(t *testing.T) {
	logData := `[sign] >>>>>> 开始主账号签到 (cookie.json) <<<<<<
  🎉 成功领取云贝 [分享歌曲] 任务奖励，获得云贝: 100
  🎉 预约活动奖励领取成功，获得云贝数量：50
  [云贝余额] 当前可用云贝: 1230
`
	rewards := parseRunRewardsReader(strings.NewReader(logData))
	if len(rewards) != 1 || rewards[0].Yunbei != 150 || !rewards[0].YunbeiKnown || rewards[0].YunbeiCumulative || rewards[0].YunbeiBalance != 1230 {
		t.Fatalf("unexpected legacy rewards: %+v", rewards)
	}
}

func TestParseRunRewardsPrefersStructuredDomainEvents(t *testing.T) {
	logData := `[sign] >>>>>> 开始主账号签到 (cookie.json) <<<<<<
  [NCMM_REWARD] {"version":1,"account":"cookie.json","domain":"yunbei","yunbei":{"today":500,"todayKnown":true,"balance":1230,"balanceKnown":true}}
  [云贝收益] 今日 +500 | 当前余额 1230
  [NCMM_REWARD] {"version":1,"account":"cookie.json","domain":"vip","vip":{"enabled":true,"statusKnown":true,"today":42,"total":946,"growthKnown":true}}
  [会员收益] 今日成长值 42 | 当前成长值 946
  [今日收益] 云贝 +999 / 999 | 成长值 99 / 999
`
	rewards := parseRunRewardsReader(strings.NewReader(logData))
	if len(rewards) != 1 {
		t.Fatalf("reward count = %d; want 1: %+v", len(rewards), rewards)
	}
	reward := rewards[0]
	if reward.Account != "cookie.json" || reward.Yunbei != 500 || !reward.YunbeiKnown || !reward.YunbeiCumulative || reward.YunbeiBalance != 1230 || !reward.YunbeiBalanceKnown {
		t.Fatalf("unexpected structured yunbei reward: %+v", reward)
	}
	if !reward.VIPKnown || !reward.VIP || !reward.GrowthKnown || reward.GrowthToday != 42 || reward.GrowthTotal != 946 {
		t.Fatalf("unexpected structured VIP reward: %+v", reward)
	}
}

func TestParseRunRewardsStructuredEventCreatesAccount(t *testing.T) {
	logData := `[NCMM_REWARD] {"version":1,"account":"sub/fan1.json","domain":"yunbei","yunbei":{"today":12,"todayKnown":true,"balance":345,"balanceKnown":true}}
`
	rewards := parseRunRewardsReader(strings.NewReader(logData))
	if len(rewards) != 1 || rewards[0].Account != "sub/fan1.json" || rewards[0].Yunbei != 12 || rewards[0].YunbeiBalance != 345 {
		t.Fatalf("unexpected rewards: %+v", rewards)
	}
}

func TestHideStructuredRewardEvents(t *testing.T) {
	data := []byte("普通日志\n  [NCMM_REWARD] {\"version\":1,\"account\":\"cookie.json\",\"domain\":\"vip\",\"vip\":{}}\n收益完成\n")
	if got := string(hideStructuredRewardEvents(data)); got != "普通日志\n收益完成\n" {
		t.Fatalf("visible log = %q", got)
	}
}

func newControlledRunManager(t *testing.T, maxParallel int) *runManager {
	t.Helper()
	home := t.TempDir()
	configPath := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(configPath, configForTest(), 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := newRunManager(os.Args[0], configPath, home,
		LogPolicy{RetentionDays: 7, MaxTotalSizeMB: 64},
		ConcurrencyPolicy{MaxParallel: maxParallel},
	)
	if err != nil {
		t.Fatal(err)
	}
	manager.command = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		gate := flagValue(args, "--test-gate")
		return exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunManagerHelperProcess$", "--", "--run-manager-helper", gate)
	}
	return manager
}

func controlledJob(id, policy, account, gate string) Schedule {
	return Schedule{
		ID: id, Name: id, Command: "sign", OverlapPolicy: policy, Source: "test",
		Args: []string{"--cookie-file", account, "--test-gate", gate},
	}
}

func releaseRun(t *testing.T, gate string) {
	t.Helper()
	if err := os.WriteFile(gate, []byte("release"), 0600); err != nil {
		t.Fatal(err)
	}
}

func waitForRunStatus(t *testing.T, manager *runManager, id string, statuses ...string) RunRecord {
	t.Helper()
	wanted := make(map[string]bool, len(statuses))
	for _, status := range statuses {
		wanted[status] = true
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if record, ok := manager.get(id); ok && wanted[record.Status] {
			return record
		}
		time.Sleep(20 * time.Millisecond)
	}
	record, _ := manager.get(id)
	t.Fatalf("run %s status = %q; want one of %v", id, record.Status, statuses)
	return RunRecord{}
}

func TestRunManagerAllowTracksOutOfOrderRuns(t *testing.T) {
	manager := newControlledRunManager(t, 2)
	gateFirst := filepath.Join(manager.home, "first.done")
	gateSecond := filepath.Join(manager.home, "second.done")
	first, err := manager.start(context.Background(), controlledJob("same-job", "allow", "account-a.json", gateFirst))
	if err != nil || first.Status != "running" {
		t.Fatalf("first start = %+v, %v", first, err)
	}
	second, err := manager.start(context.Background(), controlledJob("same-job", "allow", "account-b.json", gateSecond))
	if err != nil || second.Status != "running" {
		t.Fatalf("second start = %+v, %v", second, err)
	}

	releaseRun(t, gateSecond)
	waitForRunStatus(t, manager, second.ID, "success")
	if running, queued := manager.jobActivity("same-job"); !running || queued != 0 {
		t.Fatalf("activity after out-of-order completion = running:%v queued:%d", running, queued)
	}
	releaseRun(t, gateFirst)
	waitForRunStatus(t, manager, first.ID, "success")
	if running, queued := manager.jobActivity("same-job"); running || queued != 0 {
		t.Fatalf("final activity = running:%v queued:%d", running, queued)
	}
}

func TestRunManagerAllowQueuesSameResourceAndPolicySwitch(t *testing.T) {
	manager := newControlledRunManager(t, 2)
	gateFirst := filepath.Join(manager.home, "first.done")
	gateSecond := filepath.Join(manager.home, "second.done")
	first, _ := manager.start(context.Background(), controlledJob("same-job", "skip", "shared.json", gateFirst))
	second, err := manager.start(context.Background(), controlledJob("same-job", "allow", "shared.json", gateSecond))
	if err != nil || first.Status != "running" || second.Status != "queued" {
		t.Fatalf("policy switch states: first=%s second=%s err=%v", first.Status, second.Status, err)
	}
	if running, queued := manager.jobActivity("same-job"); !running || queued != 1 {
		t.Fatalf("policy switch activity = running:%v queued:%d", running, queued)
	}
	releaseRun(t, gateFirst)
	waitForRunStatus(t, manager, second.ID, "running")
	releaseRun(t, gateSecond)
	waitForRunStatus(t, manager, second.ID, "success")
}

func TestRunManagerIncreasingLimitDrainsQueue(t *testing.T) {
	manager := newControlledRunManager(t, 1)
	gateA := filepath.Join(manager.home, "a.done")
	gateB := filepath.Join(manager.home, "b.done")
	a, _ := manager.start(context.Background(), controlledJob("job-a", "skip", "a.json", gateA))
	b, _ := manager.start(context.Background(), controlledJob("job-b", "skip", "b.json", gateB))
	if a.Status != "running" || b.Status != "queued" {
		t.Fatalf("initial states: a=%s b=%s", a.Status, b.Status)
	}
	manager.setConcurrencyPolicy(ConcurrencyPolicy{MaxParallel: 2})
	waitForRunStatus(t, manager, b.ID, "running")
	if manager.runningCount() != 2 {
		t.Fatalf("running count after limit increase = %d; want 2", manager.runningCount())
	}
	releaseRun(t, gateA)
	releaseRun(t, gateB)
	waitForRunStatus(t, manager, a.ID, "success")
	waitForRunStatus(t, manager, b.ID, "success")
}

func TestRunManagerSerializesSharedAccountAndAuditsSkip(t *testing.T) {
	manager := newControlledRunManager(t, 2)
	gateFirst := filepath.Join(manager.home, "first.done")
	gateSecond := filepath.Join(manager.home, "second.done")
	first, err := manager.start(context.Background(), controlledJob("job-a", "skip", "shared.json", gateFirst))
	if err != nil || first.Status != "running" {
		t.Fatalf("first start = %+v, %v", first, err)
	}
	second, err := manager.start(context.Background(), controlledJob("job-b", "skip", "shared.json", gateSecond))
	if err != nil || second.Status != "queued" {
		t.Fatalf("shared-resource start = %+v, %v; want queued", second, err)
	}
	duplicate, err := manager.start(context.Background(), controlledJob("job-a", "skip", "other.json", filepath.Join(manager.home, "duplicate.done")))
	if err != nil || duplicate.Status != "skipped" || !strings.Contains(duplicate.Error, "已有活动运行") {
		t.Fatalf("duplicate start = %+v, %v; want audited skip", duplicate, err)
	}
	if logData, err := manager.readLog(duplicate.ID); err != nil || !strings.Contains(string(logData), "skipped") {
		t.Fatalf("skip log = %q, %v", logData, err)
	}

	releaseRun(t, gateFirst)
	waitForRunStatus(t, manager, first.ID, "success")
	waitForRunStatus(t, manager, second.ID, "running")
	releaseRun(t, gateSecond)
	waitForRunStatus(t, manager, second.ID, "success")
}

func TestRunManagerParallelAccountsAndDatabaseMutex(t *testing.T) {
	t.Run("different accounts", func(t *testing.T) {
		manager := newControlledRunManager(t, 2)
		gateA := filepath.Join(manager.home, "a.done")
		gateB := filepath.Join(manager.home, "b.done")
		a, _ := manager.start(context.Background(), controlledJob("job-a", "skip", "a.json", gateA))
		b, _ := manager.start(context.Background(), controlledJob("job-b", "skip", "b.json", gateB))
		if a.Status != "running" || b.Status != "running" || manager.runningCount() != 2 {
			t.Fatalf("different accounts did not run in parallel: a=%s b=%s count=%d", a.Status, b.Status, manager.runningCount())
		}
		releaseRun(t, gateA)
		releaseRun(t, gateB)
		waitForRunStatus(t, manager, a.ID, "success")
		waitForRunStatus(t, manager, b.ID, "success")
	})

	t.Run("shared database", func(t *testing.T) {
		manager := newControlledRunManager(t, 2)
		gateA := filepath.Join(manager.home, "a.done")
		gateB := filepath.Join(manager.home, "b.done")
		jobA := controlledJob("job-a", "skip", "a.json", gateA)
		jobB := controlledJob("job-b", "skip", "b.json", gateB)
		jobA.Command = "playids"
		jobB.Command = "playids"
		a, _ := manager.start(context.Background(), jobA)
		b, _ := manager.start(context.Background(), jobB)
		if a.Status != "running" || b.Status != "queued" {
			t.Fatalf("database mutex states: a=%s b=%s", a.Status, b.Status)
		}
		releaseRun(t, gateA)
		waitForRunStatus(t, manager, b.ID, "running")
		releaseRun(t, gateB)
		waitForRunStatus(t, manager, b.ID, "success")
	})
}

func TestRunManagerStopsQueuedRun(t *testing.T) {
	manager := newControlledRunManager(t, 1)
	gateFirst := filepath.Join(manager.home, "first.done")
	first, _ := manager.start(context.Background(), controlledJob("job-a", "skip", "a.json", gateFirst))
	queued, _ := manager.start(context.Background(), controlledJob("job-b", "skip", "b.json", filepath.Join(manager.home, "second.done")))
	if queued.Status != "queued" {
		t.Fatalf("second status = %s; want queued", queued.Status)
	}
	if err := manager.stop(queued.ID); err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, manager, queued.ID, "stopped")
	if _, count := manager.jobActivity("job-b"); count != 0 {
		t.Fatalf("stopped queue still active: %d", count)
	}
	releaseRun(t, gateFirst)
	waitForRunStatus(t, manager, first.ID, "success")
}

func TestRunManagerRecordsExplicitStop(t *testing.T) {
	manager := newControlledRunManager(t, 1)
	gate := filepath.Join(manager.home, "never-release.done")
	record, err := manager.start(context.Background(), controlledJob("job-stop", "skip", "account.json", gate))
	if err != nil || record.Status != "running" {
		t.Fatalf("start = %+v, %v", record, err)
	}
	if err := manager.stop(record.ID); err != nil {
		t.Fatal(err)
	}
	stopped := waitForRunStatus(t, manager, record.ID, "stopped")
	if stopped.ExitCode == nil || *stopped.ExitCode != -1 || !strings.Contains(stopped.Error, "已停止") {
		t.Fatalf("unexpected stopped record: %+v", stopped)
	}
}

func TestRunManagerDeletesOnlyFinishedRunFiles(t *testing.T) {
	manager := newControlledRunManager(t, 1)
	gate := filepath.Join(manager.home, "delete.done")
	record, err := manager.start(context.Background(), controlledJob("job-delete", "skip", "account.json", gate))
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.deleteRun(record.ID); err == nil || !strings.Contains(err.Error(), "cannot be deleted") {
		t.Fatalf("delete running error = %v", err)
	}
	releaseRun(t, gate)
	finished := waitForRunStatus(t, manager, record.ID, "success")
	if _, err := os.Stat(finished.LogFile); err != nil {
		t.Fatalf("log file before delete: %v", err)
	}
	if _, err := os.Stat(finished.MetaFile); err != nil {
		t.Fatalf("meta file before delete: %v", err)
	}
	if err := manager.deleteRun(record.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := manager.get(record.ID); ok {
		t.Fatal("deleted run remains in memory")
	}
	for _, path := range []string{finished.LogFile, finished.MetaFile} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("deleted file %s still exists: %v", path, err)
		}
	}
}

func TestRunManagerRecordsStartFailure(t *testing.T) {
	manager := newControlledRunManager(t, 1)
	manager.command = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, filepath.Join(t.TempDir(), "missing-command"))
	}
	record, err := manager.start(context.Background(), controlledJob("job-fail", "skip", "account.json", "unused"))
	if err == nil || record.Status != "failed" || record.ExitCode == nil || *record.ExitCode != -1 || !strings.Contains(record.Error, "start task process") {
		t.Fatalf("start failure record = %+v, %v", record, err)
	}
	stored, ok := manager.get(record.ID)
	if !ok || stored.Status != "failed" || manager.runningCount() != 0 {
		t.Fatalf("stored start failure = %+v, running=%d", stored, manager.runningCount())
	}
}

func TestRunManagerSupervisorShutdownStopsRunningTask(t *testing.T) {
	manager := newControlledRunManager(t, 1)
	record, err := manager.start(context.Background(), controlledJob("job-shutdown", "skip", "account.json", filepath.Join(manager.home, "never.done")))
	if err != nil || record.Status != "running" {
		t.Fatalf("start = %+v, %v", record, err)
	}
	manager.supervisor.Close(time.Second)
	stopped := waitForRunStatus(t, manager, record.ID, "stopped")
	if stopped.ExitCode == nil || *stopped.ExitCode != -1 || manager.runningCount() != 0 {
		t.Fatalf("shutdown record = %+v, running=%d", stopped, manager.runningCount())
	}
}
