package rewardevent

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	Prefix  = "[NCMM_REWARD] "
	Version = 1

	DomainYunbei = "yunbei"
	DomainVIP    = "vip"
)

type Yunbei struct {
	Today        int64 `json:"today"`
	TodayKnown   bool  `json:"todayKnown"`
	Balance      int64 `json:"balance"`
	BalanceKnown bool  `json:"balanceKnown"`
}

type VIP struct {
	Enabled     bool  `json:"enabled"`
	StatusKnown bool  `json:"statusKnown"`
	Today       int64 `json:"today"`
	Total       int64 `json:"total"`
	GrowthKnown bool  `json:"growthKnown"`
}

// Event 是 CLI 子进程与 WebUI 之间的稳定收益数据契约，展示文案可以独立调整。
type Event struct {
	Version int     `json:"version"`
	Account string  `json:"account"`
	Domain  string  `json:"domain"`
	Yunbei  *Yunbei `json:"yunbei,omitempty"`
	VIP     *VIP    `json:"vip,omitempty"`
}

func MarshalLine(event Event) (string, error) {
	if event.Version == 0 {
		event.Version = Version
	}
	if err := validate(event); err != nil {
		return "", err
	}
	data, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("marshal reward event: %w", err)
	}
	return Prefix + string(data), nil
}

// ParseLine 仅在识别到收益事件前缀时返回 matched=true。
func ParseLine(line string) (event Event, matched bool, err error) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, Prefix) {
		return Event{}, false, nil
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, Prefix))), &event); err != nil {
		return Event{}, true, fmt.Errorf("decode reward event: %w", err)
	}
	if err := validate(event); err != nil {
		return Event{}, true, err
	}
	return event, true, nil
}

func validate(event Event) error {
	if event.Version != Version {
		return fmt.Errorf("unsupported reward event version: %d", event.Version)
	}
	if strings.TrimSpace(event.Account) == "" {
		return fmt.Errorf("reward event account is required")
	}
	switch event.Domain {
	case DomainYunbei:
		if event.Yunbei == nil || event.VIP != nil {
			return fmt.Errorf("yunbei reward event has invalid payload")
		}
	case DomainVIP:
		if event.VIP == nil || event.Yunbei != nil {
			return fmt.Errorf("vip reward event has invalid payload")
		}
	default:
		return fmt.Errorf("unsupported reward event domain: %q", event.Domain)
	}
	return nil
}
