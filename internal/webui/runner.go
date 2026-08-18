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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type runningProcess struct {
	cancel context.CancelFunc
	cmd    *exec.Cmd
}

type runManager struct {
	mu         sync.RWMutex
	executable string
	configPath string
	home       string
	logDir     string
	runs       map[string]RunRecord
	running    map[string]*runningProcess
	jobRuns    map[string]string
	policy     LogPolicy
}

func newRunManager(executable, configPath, home string, policy LogPolicy) (*runManager, error) {
	logDir := filepath.Join(home, "log", "runs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create run log directory: %w", err)
	}
	m := &runManager{
		executable: executable,
		configPath: configPath,
		home:       home,
		logDir:     logDir,
		runs:       make(map[string]RunRecord),
		running:    make(map[string]*runningProcess),
		jobRuns:    make(map[string]string),
		policy:     policy,
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

func (m *runManager) start(ctx context.Context, job Schedule) (RunRecord, error) {
	m.mu.Lock()
	if existingID := m.jobRuns[job.ID]; existingID != "" && job.OverlapPolicy != "allow" {
		existing := m.runs[existingID]
		m.mu.Unlock()
		return existing, fmt.Errorf("job is already running")
	}

	runID := newID("run")
	stamp := time.Now().Format("20060102-150405")
	base := stamp + "_" + sanitizeFilePart(job.ID) + "_" + runID
	logPath := filepath.Join(m.logDir, base+".log")
	metaPath := filepath.Join(m.logDir, base+".json")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		m.mu.Unlock()
		return RunRecord{}, err
	}

	args := []string{"--config", m.configPath, "--home", m.home, job.Command}
	args = append(args, job.Args...)
	runCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(runCtx, m.executable, args...)
	cmd.Dir = m.home
	cmd.Env = append(os.Environ(), "NCMM_WEB_CHILD=1")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	record := RunRecord{
		ID:        runID,
		JobID:     job.ID,
		JobName:   job.Name,
		Command:   job.Command,
		Args:      append([]string(nil), job.Args...),
		Status:    "running",
		Source:    job.Source,
		StartedAt: time.Now(),
		LogFile:   logPath,
		MetaFile:  metaPath,
	}
	m.runs[runID] = record
	m.running[runID] = &runningProcess{cancel: cancel, cmd: cmd}
	m.jobRuns[job.ID] = runID
	m.writeMetaLocked(record)

	if err := cmd.Start(); err != nil {
		cancel()
		logFile.Close()
		delete(m.running, runID)
		delete(m.jobRuns, job.ID)
		record.Status = "failed"
		record.Error = err.Error()
		finished := time.Now()
		record.FinishedAt = &finished
		m.runs[runID] = record
		m.writeMetaLocked(record)
		m.mu.Unlock()
		return record, err
	}
	m.mu.Unlock()

	go m.wait(runID, job.ID, cmd, logFile)
	return record, nil
}

func (m *runManager) wait(runID, jobID string, cmd *exec.Cmd, logFile *os.File) {
	err := cmd.Wait()
	_ = logFile.Close()

	m.mu.Lock()
	record := m.runs[runID]
	finished := time.Now()
	record.FinishedAt = &finished
	exitCode := 0
	if err != nil {
		record.Status = "failed"
		record.Error = err.Error()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	} else {
		record.Status = "success"
	}
	if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == -1 {
		record.Status = "stopped"
		exitCode = -1
	}
	record.ExitCode = &exitCode
	m.runs[runID] = record
	delete(m.running, runID)
	if m.jobRuns[jobID] == runID {
		delete(m.jobRuns, jobID)
	}
	m.writeMetaLocked(record)
	m.mu.Unlock()
	_ = m.cleanup()
}

func (m *runManager) stop(id string) error {
	m.mu.RLock()
	process := m.running[id]
	m.mu.RUnlock()
	if process == nil {
		return os.ErrNotExist
	}
	process.cancel()
	return nil
}

func (m *runManager) isJobRunning(jobID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.jobRuns[jobID] != ""
}

func (m *runManager) runningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.running)
}

func (m *runManager) list() []RunRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]RunRecord, 0, len(m.runs))
	for _, record := range m.runs {
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].StartedAt.After(result[j].StartedAt) })
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
	_ = os.WriteFile(record.MetaFile, append(data, '\n'), 0600)
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
		if record.Status == "running" {
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
