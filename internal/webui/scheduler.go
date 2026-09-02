package webui

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var allowedCommands = map[string]bool{
	"task": true, "sign": true, "playids": true,
	"musician": true, "musician-sign": true, "musician-vip": true,
	"note": true, "daily-song-share": true, "vip-member-gift": true,
	"fansgroup": true,
}

type scheduler struct {
	mu     sync.RWMutex
	active bool
	ctx    context.Context
	store  *webConfigStore
	runner *runManager
	cron   *cron.Cron
	parser cron.Parser
	jobs   map[string]Schedule
}

func newScheduler(ctx context.Context, store *webConfigStore, runner *runManager) (*scheduler, error) {
	s := &scheduler{
		ctx:    ctx,
		store:  store,
		runner: runner,
		parser: cron.NewParser(
			cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
		),
		jobs: make(map[string]Schedule),
	}
	if err := s.reload(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *scheduler) reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.store.snapshot()
	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}
	if s.cron != nil {
		s.cron.Stop()
	}
	s.cron = cron.New(cron.WithLocation(location), cron.WithParser(s.parser))
	s.jobs = make(map[string]Schedule)
	jobs := append(cfg.Jobs, environmentSchedules()...)
	for _, job := range jobs {
		normalizeSchedule(&job)
		if err := validateSchedule(job, s.parser); err != nil {
			if job.Source == "environment" {
				continue
			}
			return fmt.Errorf("schedule %q: %w", job.Name, err)
		}
		s.jobs[job.ID] = job
		if job.Enabled {
			jobCopy := job
			if _, err := s.cron.AddFunc(job.Cron, func() { _, _ = s.runner.start(s.ctx, jobCopy) }); err != nil {
				return err
			}
		}
	}
	if s.active {
		s.cron.Start()
	}
	return nil
}

func (s *scheduler) start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		return
	}
	s.active = true
	if s.cron != nil {
		s.cron.Start()
	}
}

func validateSchedule(job Schedule, parser cron.Parser) error {
	if strings.TrimSpace(job.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if !allowedCommands[job.Command] {
		return fmt.Errorf("unsupported command %q", job.Command)
	}
	if _, err := parser.Parse(job.Cron); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}
	if job.OverlapPolicy != "skip" && job.OverlapPolicy != "allow" {
		return fmt.Errorf("overlapPolicy must be skip or allow")
	}
	return nil
}

func (s *scheduler) list() []ScheduleView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := s.store.snapshot()
	location, _ := time.LoadLocation(cfg.Timezone)
	now := time.Now().In(location)
	result := make([]ScheduleView, 0, len(s.jobs))
	for _, job := range s.jobs {
		running, queued := s.runner.jobActivity(job.ID)
		view := ScheduleView{Schedule: job, Running: running, Queued: queued}
		if job.Enabled {
			schedule, err := s.parser.Parse(job.Cron)
			if err != nil {
				continue
			}
			next := now
			for range 5 {
				next = schedule.Next(next)
				view.NextRuns = append(view.NextRuns, next)
			}
		}
		result = append(result, view)
	}
	configuredOrder := make(map[string]int, len(cfg.Jobs))
	for index, job := range cfg.Jobs {
		configuredOrder[job.ID] = index
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Enabled != result[j].Enabled {
			return result[i].Enabled
		}
		if result[i].Pinned != result[j].Pinned {
			return result[i].Pinned
		}
		leftOrder, leftConfigured := configuredOrder[result[i].ID]
		rightOrder, rightConfigured := configuredOrder[result[j].ID]
		if leftConfigured != rightConfigured {
			return leftConfigured
		}
		if leftConfigured && leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		if result[i].ReadOnly != result[j].ReadOnly {
			return !result[i].ReadOnly
		}
		return result[i].Name < result[j].Name
	})
	return result
}

func (s *scheduler) get(id string) (Schedule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	return job, ok
}

func (s *scheduler) upsert(job Schedule) (Schedule, error) {
	if job.ReadOnly || job.Source == "environment" {
		return Schedule{}, fmt.Errorf("environment schedules are read-only")
	}
	normalizeSchedule(&job)
	if err := validateSchedule(job, s.parser); err != nil {
		return Schedule{}, err
	}
	saved, err := s.store.upsert(job)
	if err != nil {
		return Schedule{}, err
	}
	if err := s.reload(); err != nil {
		return Schedule{}, err
	}
	return saved, nil
}

func (s *scheduler) delete(id string) error {
	job, ok := s.get(id)
	if !ok {
		return os.ErrNotExist
	}
	if job.ReadOnly {
		return fmt.Errorf("environment schedules are read-only")
	}
	if err := s.store.delete(id); err != nil {
		return err
	}
	return s.reload()
}

func (s *scheduler) pin(id string) error {
	job, ok := s.get(id)
	if !ok {
		return os.ErrNotExist
	}
	if job.ReadOnly {
		return fmt.Errorf("environment schedules are read-only")
	}
	if err := s.store.pin(id); err != nil {
		return err
	}
	return s.reload()
}

func (s *scheduler) runNow(id string) (RunRecord, error) {
	job, ok := s.get(id)
	if !ok {
		return RunRecord{}, os.ErrNotExist
	}
	return s.runner.start(s.ctx, job)
}

func (s *scheduler) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
	if s.cron != nil {
		s.cron.Stop()
	}
}

func (s *scheduler) isActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

func (s *scheduler) timezone() string {
	return s.store.snapshot().Timezone
}
