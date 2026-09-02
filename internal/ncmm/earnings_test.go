package ncmm

import (
	"testing"
	"time"

	"github.com/3899/ncmm/api/eapi"
)

func TestYunbeiIncomeOnDate(t *testing.T) {
	entries := []eapi.YunbeiReceiptData{
		{Date: "2026-09-01", PointCost: 300},
		{Date: "2026-09-01", PointCost: 100},
		{Date: "2026-08-31", PointCost: 50},
	}
	if got := yunbeiIncomeOnDate(entries, "2026-09-01"); got != 400 {
		t.Fatalf("yunbeiIncomeOnDate() = %d; want 400", got)
	}
}

func TestVipGrowthOnDate(t *testing.T) {
	details := []eapi.VipGrowthDetail{
		{Time: time.Date(2026, 9, 1, 0, 1, 0, 0, neteaseLocation).UnixMilli(), GrowthPoint: 6},
		{Time: time.Date(2026, 9, 1, 23, 59, 0, 0, neteaseLocation).UnixMilli(), GrowthPoint: 3},
		{Time: time.Date(2026, 8, 31, 23, 59, 0, 0, neteaseLocation).UnixMilli(), GrowthPoint: 20},
	}
	if got := vipGrowthOnDate(details, "2026-09-01"); got != 9 {
		t.Fatalf("vipGrowthOnDate() = %d; want 9", got)
	}
}

func TestParseVipGrowthExt(t *testing.T) {
	today, month, day, ok := parseVipGrowthExt(`{"todayScore":12,"monthTaskTotalScore":6,"currentDay":"202691"}`)
	if !ok || today != 12 || month != 6 || day != "202691" {
		t.Fatalf("unexpected parsed ext: today=%d month=%d day=%q ok=%v", today, month, day, ok)
	}
	for _, raw := range []string{"", `{}`, `{"todayScore":`} {
		if _, _, _, ok := parseVipGrowthExt(raw); ok {
			t.Fatalf("invalid ext %q was accepted", raw)
		}
	}
}

func TestEarningsValue(t *testing.T) {
	if got := earningsValue(true, 0, "+"); got != "+0" {
		t.Fatalf("known zero = %q; want +0", got)
	}
	if got := earningsValue(false, 100, "+"); got != "--" {
		t.Fatalf("unknown = %q; want --", got)
	}
}

func TestDomainEarningsLines(t *testing.T) {
	yunbei := yunbeiEarnings{Today: 500, TodayKnown: true, Balance: 1230, BalanceKnown: true}
	if got := yunbeiEarningsLine(yunbei); got != "  [云贝收益] 今日 +500 | 当前余额 1230" {
		t.Fatalf("unexpected yunbei line: %q", got)
	}
	vip := vipEarnings{Enabled: true, StatusKnown: true, Today: 42, Total: 946, GrowthKnown: true}
	if got := vipEarningsLine(vip); got != "  [会员收益] 今日成长值 42 | 当前成长值 946" {
		t.Fatalf("unexpected VIP line: %q", got)
	}
	if got := vipEarningsLine(vipEarnings{StatusKnown: true}); got != "  [会员收益] 非 VIP 账号" {
		t.Fatalf("unexpected non-VIP line: %q", got)
	}
}

func TestDailyEarningsLineUsesCollectedDomainsOnly(t *testing.T) {
	yunbei := yunbeiEarnings{Today: 500, TodayKnown: true, Balance: 1230, BalanceKnown: true}
	report := accountEarningsReport{Account: "cookie.json", Yunbei: &yunbei}
	if got := dailyEarningsLine(report); got != "  [今日收益] 云贝 +500 / 1230" {
		t.Fatalf("yunbei-only summary = %q", got)
	}

	nonVIP := vipEarnings{StatusKnown: true}
	report.VIP = &nonVIP
	if got := dailyEarningsLine(report); got != "  [今日收益] 云贝 +500 / 1230 | 非 VIP 账号" {
		t.Fatalf("non-VIP summary = %q", got)
	}

	vip := vipEarnings{Enabled: true, StatusKnown: true, Today: 42, Total: 946, GrowthKnown: true}
	report.VIP = &vip
	if got := dailyEarningsLine(report); got != "  [今日收益] 云贝 +500 / 1230 | 成长值 42 / 946" {
		t.Fatalf("VIP summary = %q", got)
	}
}

func TestAccountEarningsReportCollectsEachDomainOnce(t *testing.T) {
	report := accountEarningsReport{Account: "cookie.json"}
	yunbeiCalls := 0
	for range 2 {
		got := report.collectYunbei(func() yunbeiEarnings {
			yunbeiCalls++
			return yunbeiEarnings{Today: int64(yunbeiCalls), TodayKnown: true}
		})
		if got.Today != 1 {
			t.Fatalf("cached yunbei value = %d; want 1", got.Today)
		}
	}
	if yunbeiCalls != 1 {
		t.Fatalf("yunbei fetch count = %d; want 1", yunbeiCalls)
	}

	vipCalls := 0
	for range 2 {
		got := report.collectVIP(func() vipEarnings {
			vipCalls++
			return vipEarnings{Enabled: true, StatusKnown: true, Today: int64(vipCalls), GrowthKnown: true}
		})
		if got.Today != 1 {
			t.Fatalf("cached VIP value = %d; want 1", got.Today)
		}
	}
	if vipCalls != 1 {
		t.Fatalf("VIP fetch count = %d; want 1", vipCalls)
	}
}

func TestQueueHasActiveSignTasks(t *testing.T) {
	task := &Task{}
	active := map[string]bool{"sign": true}
	if !task.queueHasActiveSignTasks([]string{"playids", "ListenIndie"}, active) {
		t.Fatal("slow queue sign task was not detected")
	}
	if task.queueHasActiveSignTasks([]string{"playids", "musician-vip"}, active) {
		t.Fatal("unrelated slow queue was treated as a sign queue")
	}
	if task.queueHasActiveSignTasks([]string{"VipTask"}, map[string]bool{}) {
		t.Fatal("disabled sign task was treated as active")
	}
}
