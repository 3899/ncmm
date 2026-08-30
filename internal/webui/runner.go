package webui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/3899/ncmm/config"
	"github.com/3899/ncmm/internal/filelock"
)

const maxQueuedRuns = 100

var databaseCommands = map[string]bool{
	"task": true, "playids": true, "musician": true,
	"musician-vip": true, "vip-member-gift": true,
}

type runningProcess struct {
	process   *managedProcess
	jobID     string
	resources []string
}

type queuedProcess struct {
	ctx       context.Context
	job       Schedule
	resources []string
}

type runCommandFactory func(context.Context, string, ...string) *exec.Cmd

type runManager struct {
	mu          sync.RWMutex
	executable  string
	configPath  string
	home        string
	logDir      string
	runs        map[string]RunRecord
	running     map[string]*runningProcess
	queued      map[string]*queuedProcess
	queue       []string
	jobRuns     map[string]map[string]struct{}
	resources   map[string]string
	maxParallel int
	supervisor  *processSupervisor
	command     runCommandFactory
	policy      LogPolicy
}

func newRunManager(executable, configPath, home string, policy LogPolicy, concurrency ConcurrencyPolicy, supervisors ...*processSupervisor) (*runManager, error) {
	logDir := filepath.Join(home, "log", "runs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create run log directory: %w", err)
	}
	maxParallel := concurrency.MaxParallel
	if maxParallel < 1 || maxParallel > 8 {
		maxParallel = 1
	}
	supervisor := newProcessSupervisor()
	if len(supervisors) > 0 && supervisors[0] != nil {
		supervisor = supervisors[0]
	}
	m := &runManager{
		executable:  executable,
		configPath:  configPath,
		home:        home,
		logDir:      logDir,
		runs:        make(map[string]RunRecord),
		running:     make(map[string]*runningProcess),
		queued:      make(map[string]*queuedProcess),
		jobRuns:     make(map[string]map[string]struct{}),
		resources:   make(map[string]string),
		maxParallel: maxParallel,
		supervisor:  supervisor,
		command:     exec.CommandContext,
		policy:      policy,
	}
	m.loadHistory()
	_ = m.cleanup()
	return m, nil
}

func (m *runManager) setPolicy(policy LogPolicy) {
	m.mu.Lock()
	m.policy = policy
	m.mu.Unlock()
}

func (m *runManager) setConcurrencyPolicy(policy ConcurrencyPolicy) {
	m.mu.Lock()
	if policy.MaxParallel < 1 || policy.MaxParallel > 8 {
		policy.MaxParallel = 1
	}
	m.maxParallel = policy.MaxParallel
	m.drainQueueLocked()
	m.mu.Unlock()
}

func (m *runManager) start(ctx context.Context, job Schedule) (RunRecord, error) {
	resources := m.concurrencyKeys(ctx, job)
	m.mu.Lock()
	defer m.mu.Unlock()

	runID := newID("run")
	stamp := time.Now().Format("20060102-150405")
	base := stamp + "_" + sanitizeFilePart(job.ID) + "_" + runID
	logPath := filepath.Join(m.logDir, base+".log")
	metaPath := filepath.Join(m.logDir, base+".json")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return RunRecord{}, err
	}
	_ = logFile.Close()

	now := time.Now()
	record := RunRecord{
		ID:          runID,
		JobID:       job.ID,
		JobName:     job.Name,
		Command:     job.Command,
		Args:        append([]string(nil), job.Args...),
		Status:      "queued",
		Source:      job.Source,
		TriggeredAt: now,
		StartedAt:   now,
		Resources:   append([]string(nil), resources...),
		LogFile:     logPath,
		MetaFile:    metaPath,
	}
	m.runs[runID] = record
	if ctx.Err() != nil {
		record.Status = "stopped"
		record.Error = ctx.Err().Error()
		record.FinishedAt = &now
		m.runs[runID] = record
		m.writeMetaLocked(record)
		return record, nil
	}
	if job.OverlapPolicy != "allow" && m.jobActiveLocked(job.ID) {
		record.Status = "skipped"
		record.Error = "同一定时任务已有活动运行"
		record.FinishedAt = &now
		m.runs[runID] = record
		m.appendLogLocked(record, "[runner] skipped: "+record.Error+"\n")
		m.writeMetaLocked(record)
		return record, nil
	}
	if len(m.queued) >= maxQueuedRuns {
		record.Status = "skipped"
		record.Error = "任务队列已满"
		record.FinishedAt = &now
		m.runs[runID] = record
		m.appendLogLocked(record, "[runner] skipped: "+record.Error+"\n")
		m.writeMetaLocked(record)
		return record, nil
	}

	m.addJobRunLocked(job.ID, runID)
	if m.canLaunchLocked(resources) {
		launched, launchErr := m.launchLocked(ctx, job, record, resources)
		if launchErr != nil {
			m.drainQueueLocked()
		}
		return launched, launchErr
	}
	m.queued[runID] = &queuedProcess{ctx: ctx, job: job, resources: append([]string(nil), resources...)}
	m.queue = append(m.queue, runID)
	m.writeMetaLocked(record)
	return record, nil
}

func (m *runManager) launchLocked(ctx context.Context, job Schedule, record RunRecord, resources []string) (RunRecord, error) {
	logFile, err := os.OpenFile(record.LogFile, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return m.failBeforeStartLocked(job.ID, record, err), err
	}
	args := []string{"--config", m.configPath, "--home", m.home, job.Command}
	args = append(args, job.Args...)
	record.Status = "running"
	record.StartedAt = time.Now()
	process, err := m.supervisor.Start(ctx, processSpec{
		Kind: "task", Command: m.executable, Args: args, Dir: m.home,
		Env: append(os.Environ(), "NCMM_WEB_CHILD=1"), Stdout: logFile, Stderr: logFile,
		CommandFactory: processCommandFactory(m.command),
	})
	if err != nil {
		_ = logFile.Close()
		return m.failBeforeStartLocked(job.ID, record, err), err
	}
	m.runs[record.ID] = record
	m.running[record.ID] = &runningProcess{
		process: process, jobID: job.ID, resources: append([]string(nil), resources...),
	}
	for _, resource := range resources {
		m.resources[resource] = record.ID
	}
	m.writeMetaLocked(record)
	go m.wait(record.ID, job.ID, process, logFile)
	return record, nil
}

func (m *runManager) failBeforeStartLocked(jobID string, record RunRecord, err error) RunRecord {
	record.Status = "failed"
	record.Error = err.Error()
	finished := time.Now()
	record.FinishedAt = &finished
	exitCode := -1
	record.ExitCode = &exitCode
	m.runs[record.ID] = record
	m.removeJobRunLocked(jobID, record.ID)
	m.appendLogLocked(record, "[runner] start failed: "+err.Error()+"\n")
	m.writeMetaLocked(record)
	return record
}

func (m *runManager) wait(runID, jobID string, process *managedProcess, logFile *os.File) {
	err := process.Wait()
	_ = logFile.Close()

	m.mu.Lock()
	record := m.runs[runID]
	finished := time.Now()
	record.FinishedAt = &finished
	exitCode := 0
	switch {
	case process.StopRequested() || errors.Is(process.ContextError(), context.Canceled):
		record.Status = "stopped"
		record.Error = "任务已停止"
		exitCode = -1
	case errors.Is(process.ContextError(), context.DeadlineExceeded):
		record.Status = "timed_out"
		record.Error = context.DeadlineExceeded.Error()
		exitCode = -1
	case err != nil:
		record.Status = "failed"
		record.Error = err.Error()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	default:
		record.Status = "success"
	}
	record.ExitCode = &exitCode
	m.runs[runID] = record
	running := m.running[runID]
	delete(m.running, runID)
	if running != nil {
		m.releaseResourcesLocked(runID, running.resources)
	}
	m.removeJobRunLocked(jobID, runID)
	m.writeMetaLocked(record)
	m.drainQueueLocked()
	m.mu.Unlock()
	_ = m.cleanup()
}

func (m *runManager) stop(id string) error {
	m.mu.Lock()
	process := m.running[id]
	if process != nil {
		managed := process.process
		m.mu.Unlock()
		managed.Stop()
		return nil
	}
	pending := m.queued[id]
	if pending == nil {
		m.mu.Unlock()
		return os.ErrNotExist
	}
	delete(m.queued, id)
	m.removeJobRunLocked(pending.job.ID, id)
	record := m.runs[id]
	record.Status = "stopped"
	record.Error = "排队任务已取消"
	finished := time.Now()
	record.FinishedAt = &finished
	exitCode := -1
	record.ExitCode = &exitCode
	m.runs[id] = record
	m.appendLogLocked(record, "[runner] stopped before start\n")
	m.writeMetaLocked(record)
	m.drainQueueLocked()
	m.mu.Unlock()
	return nil
}

func (m *runManager) jobActivity(jobID string) (bool, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var running, queued int
	for runID := range m.jobRuns[jobID] {
		switch m.runs[runID].Status {
		case "running":
			running++
		case "queued":
			queued++
		}
	}
	return running > 0, queued
}

func (m *runManager) runningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.running)
}

func (m *runManager) queuedCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.queued)
}

func (m *runManager) canLaunchLocked(resources []string) bool {
	if len(m.running) >= m.maxParallel {
		return false
	}
	for _, resource := range resources {
		if m.resources[resource] != "" {
			return false
		}
	}
	return true
}

func (m *runManager) drainQueueLocked() {
	for len(m.running) < m.maxParallel {
		launched := false
		for index := 0; index < len(m.queue); index++ {
			runID := m.queue[index]
			pending := m.queued[runID]
			if pending == nil {
				m.queue = append(m.queue[:index], m.queue[index+1:]...)
				index--
				continue
			}
			if pending.ctx.Err() != nil {
				delete(m.queued, runID)
				m.queue = append(m.queue[:index], m.queue[index+1:]...)
				record := m.runs[runID]
				record.Status = "stopped"
				record.Error = pending.ctx.Err().Error()
				finished := time.Now()
				record.FinishedAt = &finished
				exitCode := -1
				record.ExitCode = &exitCode
				m.runs[runID] = record
				m.removeJobRunLocked(pending.job.ID, runID)
				m.writeMetaLocked(record)
				index--
				continue
			}
			if !m.canLaunchLocked(pending.resources) {
				continue
			}
			delete(m.queued, runID)
			m.queue = append(m.queue[:index], m.queue[index+1:]...)
			_, _ = m.launchLocked(pending.ctx, pending.job, m.runs[runID], pending.resources)
			launched = true
			break
		}
		if !launched {
			return
		}
	}
}

func (m *runManager) addJobRunLocked(jobID, runID string) {
	if m.jobRuns[jobID] == nil {
		m.jobRuns[jobID] = make(map[string]struct{})
	}
	m.jobRuns[jobID][runID] = struct{}{}
}

func (m *runManager) removeJobRunLocked(jobID, runID string) {
	delete(m.jobRuns[jobID], runID)
	if len(m.jobRuns[jobID]) == 0 {
		delete(m.jobRuns, jobID)
	}
}

func (m *runManager) jobActiveLocked(jobID string) bool {
	return len(m.jobRuns[jobID]) != 0
}

func (m *runManager) releaseResourcesLocked(runID string, resources []string) {
	for _, resource := range resources {
		if m.resources[resource] == runID {
			delete(m.resources, resource)
		}
	}
}

func (m *runManager) appendLogLocked(record RunRecord, message string) {
	file, err := os.OpenFile(record.LogFile, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	_, _ = file.WriteString(message)
	_ = file.Close()
}

func (m *runManager) concurrencyKeys(ctx context.Context, job Schedule) []string {
	var keys []string
	explicitCookie := flagValue(job.Args, "--cookie-file")
	if explicitCookie != "" {
		keys = append(keys, "account:"+m.canonicalResourcePath(explicitCookie))
	}
	needsConfig := explicitCookie == "" || databaseCommands[job.Command]
	if needsConfig {
		cfg, err := m.loadConfigForResources(ctx)
		if err != nil {
			keys = append(keys, "home:"+m.canonicalResourcePath(m.home))
		} else {
			if explicitCookie == "" && cfg.Accounts != nil {
				if cfg.Accounts.Main != "" {
					keys = append(keys, "account:"+m.canonicalResourcePath(cfg.Accounts.Main))
				}
				for _, account := range cfg.Accounts.Secondary {
					if strings.TrimSpace(account) != "" {
						keys = append(keys, "account:"+m.canonicalResourcePath(account))
					}
				}
			}
			if databaseCommands[job.Command] && cfg.Database != nil && strings.TrimSpace(cfg.Database.Path) != "" {
				keys = append(keys, "database:"+m.canonicalResourcePath(cfg.Database.Path))
			}
		}
	}
	if len(keys) == 0 {
		keys = append(keys, "home:"+m.canonicalResourcePath(m.home))
	}
	sort.Strings(keys)
	return compactStrings(keys)
}

func (m *runManager) loadConfigForResources(ctx context.Context) (*config.Config, error) {
	lockCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	lock, err := filelock.Acquire(lockCtx, m.configPath+".lock")
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	cfg, err := config.New(m.configPath)
	if err != nil {
		return nil, err
	}
	cfg.ReplaceMagicVariables("HOME", m.home)
	return cfg, nil
}

func (m *runManager) canonicalResourcePath(path string) string {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.home, path)
	}
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return filepath.ToSlash(path)
}

func flagValue(args []string, name string) string {
	for index, arg := range args {
		if arg == name && index+1 < len(args) {
			return strings.TrimSpace(args[index+1])
		}
		if value, ok := strings.CutPrefix(arg, name+"="); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func compactStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func (m *runManager) list() []RunRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]RunRecord, 0, len(m.runs))
	for _, record := range m.runs {
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := result[i].TriggeredAt, result[j].TriggeredAt
		if left.IsZero() {
			left = result[i].StartedAt
		}
		if right.IsZero() {
			right = result[j].StartedAt
		}
		return left.After(right)
	})
	if len(result) > 200 {
		result = result[:200]
	}
	return result
}

func (m *runManager) get(id string) (RunRecord, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, ok := m.runs[id]
	return record, ok
}

func (m *runManager) readLog(id string) ([]byte, error) {
	record, ok := m.get(id)
	if !ok {
		return nil, os.ErrNotExist
	}
	data, err := os.ReadFile(record.LogFile)
	if err != nil {
		return nil, err
	}
	const maxLogResponse = 2 << 20
	if len(data) > maxLogResponse {
		data = append([]byte("[showing last 2 MiB]\n"), data[len(data)-maxLogResponse:]...)
	}
	return data, nil
}

func (m *runManager) writeMetaLocked(record RunRecord) {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return
	}
	_ = writeFileAtomic(record.MetaFile, append(data, '\n'), 0600)
}

func (m *runManager) loadHistory() {
	entries, err := os.ReadDir(m.logDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		metaPath := filepath.Join(m.logDir, entry.Name())
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var record RunRecord
		if json.Unmarshal(data, &record) != nil || record.ID == "" {
			continue
		}
		record.MetaFile = metaPath
		record.LogFile = strings.TrimSuffix(metaPath, ".json") + ".log"
		if record.Status == "running" || record.Status == "queued" {
			record.Status = "interrupted"
			finished := time.Now()
			record.FinishedAt = &finished
		}
		m.runs[record.ID] = record
	}
}

func (m *runManager) cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries, err := os.ReadDir(m.logDir)
	if err != nil {
		return err
	}
	type candidate struct {
		path    string
		modTime time.Time
		size    int64
	}
	var files []candidate
	var total int64
	cutoff := time.Now().AddDate(0, 0, -m.policy.RetentionDays)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".log" && ext != ".json" {
			continue
		}
		path := filepath.Join(m.logDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) && !m.pathIsActiveLocked(path) {
			_ = os.Remove(path)
			continue
		}
		files = append(files, candidate{path: path, modTime: info.ModTime(), size: info.Size()})
		total += info.Size()
	}
	limit := m.policy.MaxTotalSizeMB * 1024 * 1024
	sort.Slice(files, func(i, j int) bool { return files[i].modTime.Before(files[j].modTime) })
	for _, file := range files {
		if total <= limit {
			break
		}
		if m.pathIsActiveLocked(file.path) {
			continue
		}
		if os.Remove(file.path) == nil {
			total -= file.size
		}
	}
	for id, record := range m.runs {
		if _, err := os.Stat(record.MetaFile); errors.Is(err, fs.ErrNotExist) {
			delete(m.runs, id)
		}
	}
	return nil
}

func (m *runManager) pathIsActiveLocked(path string) bool {
	for id := range m.running {
		record := m.runs[id]
		if record.LogFile == path || record.MetaFile == path {
			return true
		}
	}
	for id := range m.queued {
		record := m.runs[id]
		if record.LogFile == path || record.MetaFile == path {
			return true
		}
	}
	return false
}

func (m *runManager) stats() LogStats {
	entries, _ := os.ReadDir(m.logDir)
	var stats LogStats
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err == nil {
			stats.Files++
			stats.SizeBytes += info.Size()
		}
	}
	return stats
}

func sanitizeFilePart(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return strconv.FormatInt(time.Now().Unix(), 10)
	}
	return b.String()
}
