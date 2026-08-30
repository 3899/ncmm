package webui

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/3899/ncmm/internal/atomicfile"
	"gopkg.in/yaml.v3"
)

type webConfigStore struct {
	mu    sync.RWMutex
	path  string
	cfg   WebConfig
	write func(string, []byte, os.FileMode) error
}

func newWebConfigStore(path string, migrations ...*SchedulerMigration) (*webConfigStore, error) {
	s := &webConfigStore{path: path, write: writeFileAtomic}
	if err := s.load(firstSchedulerMigration(migrations)); err != nil {
		return nil, err
	}
	return s, nil
}

func defaultWebConfig() WebConfig {
	return WebConfig{
		Version:  defaultWebConfigVersion,
		Timezone: defaultTimezone,
		Logs: LogPolicy{
			RetentionDays:  defaultRetentionDays,
			MaxTotalSizeMB: defaultMaxTotalSizeMB,
		},
		Concurrency: ConcurrencyPolicy{MaxParallel: 1},
		Jobs:        []Schedule{},
	}
}

func (s *webConfigStore) load(migration *SchedulerMigration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cfg = defaultWebConfig()
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s.saveLocked()
	}
	if err != nil {
		return fmt.Errorf("read web config: %w", err)
	}
	var persisted struct {
		Version   int `yaml:"version"`
		Scheduler *struct {
			Enabled *bool `yaml:"enabled"`
		} `yaml:"scheduler"`
	}
	if err := yaml.Unmarshal(data, &persisted); err != nil {
		return fmt.Errorf("parse web config: %w", err)
	}
	if err := yaml.Unmarshal(data, &s.cfg); err != nil {
		return fmt.Errorf("parse web config: %w", err)
	}
	needsSave := persisted.Version < defaultWebConfigVersion || persisted.Scheduler != nil
	if persisted.Version < defaultWebConfigVersion {
		s.cfg.Version = defaultWebConfigVersion
		if migration == nil || !migration.PreserveEnabled {
			for i := range s.cfg.Jobs {
				s.cfg.Jobs[i].Enabled = false
			}
		}
	} else if persisted.Scheduler != nil && persisted.Scheduler.Enabled != nil && !*persisted.Scheduler.Enabled {
		for i := range s.cfg.Jobs {
			s.cfg.Jobs[i].Enabled = false
		}
	}
	normalizeWebConfig(&s.cfg)
	if needsSave {
		if err := s.saveLocked(); err != nil {
			return fmt.Errorf("migrate web config: %w", err)
		}
	}
	return nil
}

func firstSchedulerMigration(migrations []*SchedulerMigration) *SchedulerMigration {
	for _, migration := range migrations {
		if migration != nil {
			copyMigration := *migration
			return &copyMigration
		}
	}
	return nil
}

func normalizeWebConfig(cfg *WebConfig) {
	if cfg.Version == 0 {
		cfg.Version = defaultWebConfigVersion
	}
	if strings.TrimSpace(cfg.Timezone) == "" {
		cfg.Timezone = defaultTimezone
	}
	if cfg.Logs.RetentionDays <= 0 {
		cfg.Logs.RetentionDays = defaultRetentionDays
	}
	if cfg.Logs.MaxTotalSizeMB <= 0 {
		cfg.Logs.MaxTotalSizeMB = defaultMaxTotalSizeMB
	}
	if cfg.Concurrency.MaxParallel < 1 || cfg.Concurrency.MaxParallel > 8 {
		cfg.Concurrency.MaxParallel = 1
	}
	if cfg.Jobs == nil {
		cfg.Jobs = []Schedule{}
	}
	for i := range cfg.Jobs {
		normalizeSchedule(&cfg.Jobs[i])
	}
}

func normalizeSchedule(job *Schedule) {
	job.Name = strings.TrimSpace(job.Name)
	job.Cron = strings.TrimSpace(job.Cron)
	job.Command = strings.TrimSpace(job.Command)
	if job.OverlapPolicy == "" {
		job.OverlapPolicy = "skip"
	}
	if job.Source == "" {
		job.Source = "file"
	}
}

func (s *webConfigStore) snapshot() WebConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneWebConfig(s.cfg)
}

func cloneWebConfig(cfg WebConfig) WebConfig {
	copyCfg := cfg
	copyCfg.Jobs = append([]Schedule(nil), cfg.Jobs...)
	for i := range copyCfg.Jobs {
		copyCfg.Jobs[i].Args = append([]string(nil), copyCfg.Jobs[i].Args...)
	}
	return copyCfg
}

func (s *webConfigStore) updateSettings(timezone string, policy LogPolicy, concurrency ConcurrencyPolicy) (WebConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(timezone) == "" {
		timezone = defaultTimezone
	}
	if policy.RetentionDays < 1 || policy.RetentionDays > 365 {
		return WebConfig{}, fmt.Errorf("retentionDays must be between 1 and 365")
	}
	if policy.MaxTotalSizeMB < 16 || policy.MaxTotalSizeMB > 10240 {
		return WebConfig{}, fmt.Errorf("maxTotalSizeMB must be between 16 and 10240")
	}
	if concurrency.MaxParallel < 1 || concurrency.MaxParallel > 8 {
		return WebConfig{}, fmt.Errorf("maxParallel must be between 1 and 8")
	}
	next := cloneWebConfig(s.cfg)
	next.Timezone = timezone
	next.Logs = policy
	next.Concurrency = concurrency
	if err := s.saveConfigLocked(next); err != nil {
		return WebConfig{}, err
	}
	s.cfg = next
	return cloneWebConfig(s.cfg), nil
}

func (s *webConfigStore) upsert(job Schedule) (Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizeSchedule(&job)
	if job.ID == "" {
		job.ID = newID("job")
	}
	job.Source = "file"
	job.ReadOnly = false
	next := cloneWebConfig(s.cfg)
	for i := range next.Jobs {
		if next.Jobs[i].ID == job.ID {
			next.Jobs[i] = job
			if err := s.saveConfigLocked(next); err != nil {
				return Schedule{}, err
			}
			s.cfg = next
			return job, nil
		}
	}
	next.Jobs = append(next.Jobs, job)
	if err := s.saveConfigLocked(next); err != nil {
		return Schedule{}, err
	}
	s.cfg = next
	return job, nil
}

func (s *webConfigStore) delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneWebConfig(s.cfg)
	for i := range next.Jobs {
		if next.Jobs[i].ID == id {
			next.Jobs = append(next.Jobs[:i], next.Jobs[i+1:]...)
			if err := s.saveConfigLocked(next); err != nil {
				return err
			}
			s.cfg = next
			return nil
		}
	}
	return os.ErrNotExist
}

func (s *webConfigStore) saveLocked() error {
	return s.saveConfigLocked(s.cfg)
}

func (s *webConfigStore) saveConfigLocked(cfg WebConfig) error {
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("encode web config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("create web config directory: %w", err)
	}
	return s.write(s.path, data, 0600)
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	return atomicfile.Write(path, data, mode)
}

func newID(prefix string) string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, os.Getpid())
	}
	return prefix + "-" + hex.EncodeToString(raw[:])
}

func environmentSchedules() []Schedule {
	var keys []string
	values := make(map[string]string)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if !ok || !strings.HasPrefix(key, "CRON_") {
			continue
		}
		keys = append(keys, key)
		values[key] = value
	}
	sort.Strings(keys)
	jobs := make([]Schedule, 0, len(keys))
	for _, key := range keys {
		fields := strings.Fields(values[key])
		if len(fields) < 6 {
			continue
		}
		jobs = append(jobs, Schedule{
			ID:            "env-" + strings.ToLower(strings.ReplaceAll(key, "_", "-")),
			Name:          key,
			Enabled:       true,
			Cron:          strings.Join(fields[:5], " "),
			Command:       fields[5],
			Args:          fields[6:],
			OverlapPolicy: "skip",
			Source:        "environment",
			ReadOnly:      true,
		})
	}
	return jobs
}
