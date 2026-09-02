package webui

import "time"

const (
	defaultListen           = "127.0.0.1:3899"
	defaultRetentionDays    = 7
	defaultMaxTotalSizeMB   = 256
	defaultTimezone         = "Asia/Shanghai"
	defaultWebConfigVersion = 3
)

type Options struct {
	Listen             string
	Home               string
	ConfigPath         string
	WebConfig          string
	Executable         string
	Version            string
	Commit             string
	Branch             string
	BuildTime          string
	SchedulerMigration *SchedulerMigration
	SecureCookie       bool
	Output             func(string, ...any)
}

type SchedulerMigration struct {
	PreserveEnabled bool
}

type LogPolicy struct {
	RetentionEnabled bool  `json:"retentionEnabled" yaml:"retentionEnabled"`
	RetentionDays    int   `json:"retentionDays" yaml:"retentionDays"`
	MaxSizeEnabled   bool  `json:"maxSizeEnabled" yaml:"maxSizeEnabled"`
	MaxTotalSizeMB   int64 `json:"maxTotalSizeMB" yaml:"maxTotalSizeMB"`
}

type ConcurrencyPolicy struct {
	MaxParallel int `json:"maxParallel" yaml:"maxParallel"`
}

type Schedule struct {
	ID            string   `json:"id" yaml:"id"`
	Name          string   `json:"name" yaml:"name"`
	Enabled       bool     `json:"enabled" yaml:"enabled"`
	Cron          string   `json:"cron" yaml:"cron"`
	Command       string   `json:"command" yaml:"command"`
	Args          []string `json:"args" yaml:"args,omitempty"`
	OverlapPolicy string   `json:"overlapPolicy" yaml:"overlapPolicy,omitempty"`
	Source        string   `json:"source" yaml:"-"`
	ReadOnly      bool     `json:"readOnly" yaml:"-"`
	Pinned        bool     `json:"pinned,omitempty" yaml:"pinned,omitempty"`
}

type WebConfig struct {
	Version     int               `json:"version" yaml:"version"`
	Timezone    string            `json:"timezone" yaml:"timezone"`
	Logs        LogPolicy         `json:"logs" yaml:"logs"`
	Concurrency ConcurrencyPolicy `json:"concurrency" yaml:"concurrency"`
	Jobs        []Schedule        `json:"jobs" yaml:"jobs"`
}

type ScheduleView struct {
	Schedule
	NextRuns []time.Time `json:"nextRuns"`
	Running  bool        `json:"running"`
	Queued   int         `json:"queued"`
}

type RunRecord struct {
	ID          string      `json:"id"`
	JobID       string      `json:"jobId"`
	JobName     string      `json:"jobName"`
	Command     string      `json:"command"`
	Args        []string    `json:"args,omitempty"`
	Status      string      `json:"status"`
	Source      string      `json:"source"`
	TriggeredAt time.Time   `json:"triggeredAt"`
	StartedAt   time.Time   `json:"startedAt"`
	FinishedAt  *time.Time  `json:"finishedAt,omitempty"`
	ExitCode    *int        `json:"exitCode,omitempty"`
	Error       string      `json:"error,omitempty"`
	Resources   []string    `json:"resources,omitempty"`
	Rewards     []RunReward `json:"rewards,omitempty"`
	LogFile     string      `json:"-"`
	MetaFile    string      `json:"-"`
}

type RunReward struct {
	Account            string `json:"account"`
	UID                string `json:"uid,omitempty"`
	Nickname           string `json:"nickname,omitempty"`
	AvatarURL          string `json:"avatarUrl,omitempty"`
	Identity           string `json:"identity,omitempty"`
	CookieValid        bool   `json:"cookieValid,omitempty"`
	CookieKnown        bool   `json:"cookieKnown,omitempty"`
	VIP                bool   `json:"vip,omitempty"`
	VIPKnown           bool   `json:"vipKnown,omitempty"`
	Musician           bool   `json:"musician,omitempty"`
	MusicianKnown      bool   `json:"musicianKnown,omitempty"`
	Yunbei             int    `json:"yunbei"`
	YunbeiKnown        bool   `json:"yunbeiKnown,omitempty"`
	YunbeiCumulative   bool   `json:"yunbeiCumulative,omitempty"`
	YunbeiBalance      int    `json:"yunbeiBalance"`
	YunbeiBalanceKnown bool   `json:"yunbeiBalanceKnown,omitempty"`
	GrowthToday        int    `json:"growthToday"`
	GrowthTotal        int    `json:"growthTotal"`
	GrowthKnown        bool   `json:"growthKnown,omitempty"`
	EffectivePlays     int    `json:"effectivePlays"`
	EffectiveTarget    int    `json:"effectiveTarget"`
	EffectiveKnown     bool   `json:"effectiveKnown,omitempty"`
	Signed             bool   `json:"signed,omitempty"`
	SignKnown          bool   `json:"signKnown,omitempty"`
}

type LogStats struct {
	Files     int   `json:"files"`
	SizeBytes int64 `json:"sizeBytes"`
}

type LogCleanupFilter struct {
	JobName       string    `json:"jobName"`
	Statuses      []string  `json:"statuses"`
	StartedAfter  time.Time `json:"startedAfter"`
	StartedBefore time.Time `json:"startedBefore"`
}

type LogCleanupResult struct {
	Deleted    int   `json:"deleted"`
	FreedBytes int64 `json:"freedBytes"`
	Skipped    int   `json:"skipped"`
}
