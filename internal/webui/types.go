package webui

import "time"

const (
	defaultListen           = "127.0.0.1:3899"
	defaultRetentionDays    = 7
	defaultMaxTotalSizeMB   = 256
	defaultTimezone         = "Asia/Shanghai"
	defaultWebConfigVersion = 1
)

type Options struct {
	Listen           string
	Token            string
	Home             string
	ConfigPath       string
	WebConfig        string
	Executable       string
	Version          string
	Commit           string
	BuildTime        string
	Scheduler        bool
	AllowRemoteSetup bool
	Output           func(string, ...any)
}

type LogPolicy struct {
	RetentionDays  int   `json:"retentionDays" yaml:"retentionDays"`
	MaxTotalSizeMB int64 `json:"maxTotalSizeMB" yaml:"maxTotalSizeMB"`
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
}

type WebConfig struct {
	Version  int        `json:"version" yaml:"version"`
	Timezone string     `json:"timezone" yaml:"timezone"`
	Logs     LogPolicy  `json:"logs" yaml:"logs"`
	Jobs     []Schedule `json:"jobs" yaml:"jobs"`
}

type ScheduleView struct {
	Schedule
	NextRuns []time.Time `json:"nextRuns"`
	Running  bool        `json:"running"`
}

type RunRecord struct {
	ID         string     `json:"id"`
	JobID      string     `json:"jobId"`
	JobName    string     `json:"jobName"`
	Command    string     `json:"command"`
	Args       []string   `json:"args,omitempty"`
	Status     string     `json:"status"`
	Source     string     `json:"source"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	ExitCode   *int       `json:"exitCode,omitempty"`
	Error      string     `json:"error,omitempty"`
	LogFile    string     `json:"-"`
	MetaFile   string     `json:"-"`
}

type LogStats struct {
	Files     int   `json:"files"`
	SizeBytes int64 `json:"sizeBytes"`
}
