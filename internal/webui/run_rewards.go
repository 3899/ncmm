package webui

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/3899/ncmm/internal/rewardevent"
)

var (
	accountStartPattern  = regexp.MustCompile(`^\[(?:sign|musician(?: sign| vip)?)\]\s+>+\s+开始(?:主账号|辅助账号).*?\((.+)\)\s+<+$`)
	taskAccountPattern   = regexp.MustCompile(`^\[task\]\s+>+\s+账号\s+\((.+)\)\s+开始执行\s+\[[^]]+\]\s+(?:子任务组|任务)\s+<+$`)
	accountInfoPattern   = regexp.MustCompile(`\[当前账号信息\]\s*Uid:\s*([^|]+?)\s*\|\s*昵称:\s*([^|]+?)\s*\|\s*等级:\s*(.+?)(?:\s*\|\s*头像:\s*(\S+))?\s*$`)
	yunbeiRewardPattern  = regexp.MustCompile(`获得云贝(?:数量)?[：:]\s*(\d+)`)
	yunbeiBalancePattern = regexp.MustCompile(`\[云贝余额\]\s*当前可用云贝:\s*(\d+)`)
	dailyEarningsPattern = regexp.MustCompile(`^\[今日收益\]\s*云贝\s*(\+\d+|--)\s*/\s*(\d+|--)(?:\s*\|\s*(.+))?$`)
	growthSummaryPattern = regexp.MustCompile(`^成长值\s*(\d+)\s*/\s*(\d+)$`)
	growthFinalPattern   = regexp.MustCompile(`黑胶成长值最终状态:\s*今日已获得\s*(\d+)[，,]\s*当前成长值\s*(\d+)`)
	effectivePlayPattern = regexp.MustCompile(`当前进度:\s*(\d+)/(\d+)[，,]\s*今日尚缺有效播放`)
)

func parseRunRewards(path string) ([]RunReward, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return parseRunRewardsReader(file), nil
}

func parseRunRewardsReader(reader io.Reader) []RunReward {
	byAccount := make(map[string]*RunReward)
	yunbeiSummarySeen := make(map[string]bool)
	structuredYunbeiSeen := make(map[string]bool)
	structuredVIPSeen := make(map[string]bool)
	var order []string
	current := ""
	ensureAccount := func(account string) (string, *RunReward) {
		key := normalizeRewardAccount(account)
		if _, exists := byAccount[key]; !exists {
			byAccount[key] = &RunReward{Account: account}
			order = append(order, key)
		}
		return key, byAccount[key]
	}
	getCurrent := func() *RunReward {
		if current == "" {
			return nil
		}
		return byAccount[current]
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 2<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if event, matched, err := rewardevent.ParseLine(line); matched {
			if err != nil {
				continue
			}
			key, reward := ensureAccount(event.Account)
			current = key
			switch event.Domain {
			case rewardevent.DomainYunbei:
				reward.Yunbei = int(event.Yunbei.Today)
				reward.YunbeiKnown = event.Yunbei.TodayKnown
				reward.YunbeiCumulative = true
				reward.YunbeiBalance = int(event.Yunbei.Balance)
				reward.YunbeiBalanceKnown = event.Yunbei.BalanceKnown
				yunbeiSummarySeen[key] = true
				structuredYunbeiSeen[key] = true
			case rewardevent.DomainVIP:
				reward.VIP = event.VIP.Enabled
				reward.VIPKnown = event.VIP.StatusKnown
				reward.GrowthToday = int(event.VIP.Today)
				reward.GrowthTotal = int(event.VIP.Total)
				reward.GrowthKnown = event.VIP.GrowthKnown
				structuredVIPSeen[key] = true
			}
			continue
		}
		account := ""
		if match := accountStartPattern.FindStringSubmatch(line); match != nil {
			account = strings.TrimSpace(match[1])
		} else if match := taskAccountPattern.FindStringSubmatch(line); match != nil {
			account = strings.TrimSpace(match[1])
		}
		if account != "" {
			key, _ := ensureAccount(account)
			current = key
			continue
		}

		reward := getCurrent()
		if reward == nil {
			continue
		}
		if match := accountInfoPattern.FindStringSubmatch(line); match != nil {
			reward.UID = strings.TrimSpace(match[1])
			reward.Nickname = strings.TrimSpace(match[2])
			reward.Identity = strings.TrimSpace(match[3])
			if len(match) > 4 {
				reward.AvatarURL = strings.TrimSpace(match[4])
			}
			reward.CookieValid = true
			reward.CookieKnown = true
			continue
		}
		if strings.Contains(line, "Cookie 已失效") || strings.Contains(line, "Cookie失效") {
			reward.CookieValid = false
			reward.CookieKnown = true
			continue
		}
		if strings.Contains(line, "已认证音乐人") || strings.Contains(line, "[账号身份] 音乐人") {
			reward.Musician = true
			reward.MusicianKnown = true
			continue
		}
		if strings.Contains(line, "当前账号不是音乐人") {
			reward.Musician = false
			reward.MusicianKnown = true
			continue
		}
		if match := dailyEarningsPattern.FindStringSubmatch(line); match != nil {
			if !structuredYunbeiSeen[current] {
				reward.Yunbei = 0
				reward.YunbeiKnown = match[1] != "--"
				if reward.YunbeiKnown {
					reward.Yunbei, _ = strconv.Atoi(strings.TrimPrefix(match[1], "+"))
				}
				reward.YunbeiCumulative = true
				reward.YunbeiBalance = 0
				reward.YunbeiBalanceKnown = match[2] != "--"
				if reward.YunbeiBalanceKnown {
					reward.YunbeiBalance, _ = strconv.Atoi(match[2])
				}
				yunbeiSummarySeen[current] = true
			}

			if !structuredVIPSeen[current] {
				suffix := strings.TrimSpace(match[3])
				if growth := growthSummaryPattern.FindStringSubmatch(suffix); growth != nil {
					reward.GrowthToday, _ = strconv.Atoi(growth[1])
					reward.GrowthTotal, _ = strconv.Atoi(growth[2])
					reward.GrowthKnown = true
					reward.VIP = true
					reward.VIPKnown = true
				} else if suffix == "VIP 账号" {
					reward.GrowthToday = 0
					reward.GrowthTotal = 0
					reward.GrowthKnown = false
					reward.VIP = true
					reward.VIPKnown = true
				} else if suffix == "非 VIP 账号" {
					reward.GrowthToday = 0
					reward.GrowthTotal = 0
					reward.GrowthKnown = false
					reward.VIP = false
					reward.VIPKnown = true
				}
			}
			continue
		}
		if match := yunbeiRewardPattern.FindStringSubmatch(line); match != nil {
			if yunbeiSummarySeen[current] {
				continue
			}
			value, _ := strconv.Atoi(match[1])
			reward.Yunbei += value
			reward.YunbeiKnown = true
			continue
		}
		if match := yunbeiBalancePattern.FindStringSubmatch(line); match != nil {
			reward.YunbeiBalance, _ = strconv.Atoi(match[1])
			reward.YunbeiBalanceKnown = true
			continue
		}
		if strings.Contains(line, "云贝签到成功") || strings.Contains(line, "云贝今天已签到过") {
			reward.Signed = true
			reward.SignKnown = true
			continue
		}
		if strings.Contains(line, "云贝签到失败") {
			reward.SignKnown = true
			continue
		}
		if match := growthFinalPattern.FindStringSubmatch(line); match != nil && !structuredVIPSeen[current] {
			reward.GrowthToday, _ = strconv.Atoi(match[1])
			reward.GrowthTotal, _ = strconv.Atoi(match[2])
			reward.GrowthKnown = true
			reward.VIP = true
			reward.VIPKnown = true
			continue
		}
		if strings.Contains(line, "暂无会员权益") && strings.Contains(line, "VIP 状态: 0") && !structuredVIPSeen[current] {
			reward.VIP = false
			reward.VIPKnown = true
			continue
		}
		if match := effectivePlayPattern.FindStringSubmatch(line); match != nil {
			reward.EffectivePlays, _ = strconv.Atoi(match[1])
			reward.EffectiveTarget, _ = strconv.Atoi(match[2])
			reward.EffectiveKnown = true
		}
	}

	result := make([]RunReward, 0, len(order))
	for _, key := range order {
		result = append(result, *byAccount[key])
	}
	return result
}

func normalizeRewardAccount(account string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(account), `\`, "/"))
}

func hideStructuredRewardEvents(data []byte) []byte {
	lines := bytes.Split(data, []byte{'\n'})
	visible := lines[:0]
	for _, line := range lines {
		if _, matched, _ := rewardevent.ParseLine(string(line)); matched {
			continue
		}
		visible = append(visible, line)
	}
	return bytes.Join(visible, []byte{'\n'})
}
