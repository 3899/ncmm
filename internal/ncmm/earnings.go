// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package ncmm

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/3899/ncmm/api"
	"github.com/3899/ncmm/api/eapi"
	"github.com/3899/ncmm/api/types"
	"github.com/3899/ncmm/api/weapi"
	"github.com/3899/ncmm/internal/rewardevent"
	"github.com/3899/ncmm/pkg/log"
)

var neteaseLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

func yunbeiIncomeOnDate(entries []eapi.YunbeiReceiptData, date string) int64 {
	var total int64
	for _, entry := range entries {
		if entry.Date == date {
			total += entry.PointCost
		}
	}
	return total
}

func earningsValue(known bool, value int64, prefix string) string {
	if !known {
		return "--"
	}
	return prefix + fmt.Sprintf("%d", value)
}

type yunbeiEarnings struct {
	Today        int64
	TodayKnown   bool
	Balance      int64
	BalanceKnown bool
}

type vipEarnings struct {
	Enabled     bool
	StatusKnown bool
	Today       int64
	Total       int64
	GrowthKnown bool
}

type accountEarningsReport struct {
	Account string
	Yunbei  *yunbeiEarnings
	VIP     *vipEarnings
}

func (report *accountEarningsReport) collectYunbei(fetch func() yunbeiEarnings) yunbeiEarnings {
	if report.Yunbei == nil {
		value := fetch()
		report.Yunbei = &value
	}
	return *report.Yunbei
}

func (report *accountEarningsReport) collectVIP(fetch func() vipEarnings) vipEarnings {
	if report.VIP == nil {
		value := fetch()
		report.VIP = &value
	}
	return *report.VIP
}

// fetchYunbeiEarnings 在云贝域结束后读取一次收入流水和余额，不参与 VIP 数据查询。
func (c *SignIn) fetchYunbeiEarnings(
	ctx context.Context,
	cli *api.Client,
	request *weapi.Api,
	userId int64,
) yunbeiEarnings {
	deviceId := cli.GetDeviceId()
	eapiRequest := eapi.New(cli)
	result := yunbeiEarnings{}

	if userId > 0 {
		receipt, err := eapiRequest.YunbeiReceipt(ctx, &eapi.YunbeiReceiptReq{
			EApiReqCommon: types.EApiReqCommon{
				DeviceId: deviceId,
				OS:       "iOS",
				VerifyId: 1,
				Header:   struct{}{},
				ER:       true,
			},
			Limit:  50,
			Offset: 0,
			Total:  false,
			UserId: userId,
		})
		if err == nil && receipt.Code == 200 {
			result.Today = yunbeiIncomeOnDate(receipt.Data, time.Now().In(neteaseLocation).Format("2006-01-02"))
			result.TodayKnown = true
		} else if err != nil {
			c.cmd.Printf("  ⚠️ 获取今日云贝收入失败: %v\n", err)
		} else {
			c.cmd.Printf("  ⚠️ 获取今日云贝收入失败: code=%d message=%s\n", receipt.Code, strings.TrimSpace(receipt.Message))
		}
	} else {
		c.cmd.Println("  ⚠️ 未获取到账号 UID，跳过今日云贝收入查询")
	}

	if balance, err := request.YunBeiBalance(ctx, &weapi.YunBeiBalanceReq{}); err == nil && balance.Code == 200 {
		result.Balance = balance.Data.Balance
		result.BalanceKnown = true
	} else if err != nil {
		log.Debug("YunBeiBalance err: %s", err)
	}
	return result
}

// resolveVIPEarnings 优先复用 VIP 任务的最终状态，仅在缺失时补查一次。
func (c *SignIn) resolveVIPEarnings(
	ctx context.Context,
	cli *api.Client,
	vipStatus int64,
	vipStatusKnown bool,
	growth *vipGrowthState,
) vipEarnings {
	result := vipEarnings{Enabled: vipStatus == 1, StatusKnown: vipStatusKnown}
	if growth == nil && (!vipStatusKnown || vipStatus == 1) {
		if current, err := getVipGrowthState(ctx, eapi.New(cli), cli.GetDeviceId()); err == nil {
			growth = &current
		} else {
			log.Debug("VipGrowPoint earnings err: %s", err)
		}
	}
	if growth != nil {
		result.Enabled = growth.LatestVipStatus == 1
		result.StatusKnown = true
		if result.Enabled && growth.TodayScoreKnown {
			result.Today = growth.TodayScore
			result.Total = growth.GrowthPoint
			result.GrowthKnown = true
		}
	}
	return result
}

func yunbeiEarningsLine(earnings yunbeiEarnings) string {
	return fmt.Sprintf("  [云贝收益] 今日 %s | 当前余额 %s",
		earningsValue(earnings.TodayKnown, earnings.Today, "+"),
		earningsValue(earnings.BalanceKnown, earnings.Balance, ""),
	)
}

func vipEarningsLine(earnings vipEarnings) string {
	if earnings.GrowthKnown {
		return fmt.Sprintf("  [会员收益] 今日成长值 %d | 当前成长值 %d", earnings.Today, earnings.Total)
	}
	if earnings.StatusKnown && earnings.Enabled {
		return "  [会员收益] VIP 账号，成长值暂未获取"
	}
	if earnings.StatusKnown {
		return "  [会员收益] 非 VIP 账号"
	}
	return "  [会员收益] 会员状态暂未获取"
}

func dailyEarningsLine(report accountEarningsReport) string {
	line := "  [今日收益]"
	if report.Yunbei != nil {
		line += fmt.Sprintf(" 云贝 %s / %s",
			earningsValue(report.Yunbei.TodayKnown, report.Yunbei.Today, "+"),
			earningsValue(report.Yunbei.BalanceKnown, report.Yunbei.Balance, ""),
		)
	}
	if report.VIP == nil {
		return line
	}
	if report.VIP.GrowthKnown {
		line += fmt.Sprintf(" | 成长值 %d / %d", report.VIP.Today, report.VIP.Total)
	} else if report.VIP.StatusKnown && report.VIP.Enabled {
		line += " | VIP 账号"
	} else if report.VIP.StatusKnown {
		line += " | 非 VIP 账号"
	}
	return line
}

func (c *SignIn) printYunbeiEarnings(report *accountEarningsReport, earnings yunbeiEarnings) {
	c.cmd.Println(yunbeiEarningsLine(earnings))
	c.emitRewardEvent(rewardevent.Event{
		Account: report.Account,
		Domain:  rewardevent.DomainYunbei,
		Yunbei: &rewardevent.Yunbei{
			Today: earnings.Today, TodayKnown: earnings.TodayKnown,
			Balance: earnings.Balance, BalanceKnown: earnings.BalanceKnown,
		},
	})
}

func (c *SignIn) printVIPEarnings(report *accountEarningsReport, earnings vipEarnings) {
	c.cmd.Println(vipEarningsLine(earnings))
	c.emitRewardEvent(rewardevent.Event{
		Account: report.Account,
		Domain:  rewardevent.DomainVIP,
		VIP: &rewardevent.VIP{
			Enabled: earnings.Enabled, StatusKnown: earnings.StatusKnown,
			Today: earnings.Today, Total: earnings.Total, GrowthKnown: earnings.GrowthKnown,
		},
	})
}

// printDailyEarnings 只汇总账号内已经采集的数据，不持有客户端，也不会发起网络请求。
func (c *SignIn) printDailyEarnings(report accountEarningsReport) {
	c.cmd.Println(dailyEarningsLine(report))
}

func (c *SignIn) emitRewardEvent(event rewardevent.Event) {
	if os.Getenv("NCMM_WEB_CHILD") != "1" {
		return
	}
	line, err := rewardevent.MarshalLine(event)
	if err != nil {
		log.Debug("marshal reward event: %s", err)
		return
	}
	c.cmd.Println("  " + line)
}
