package webui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/3899/ncmm/internal/loginresult"
	"github.com/google/uuid"
)

const (
	qrcodeLoginTimeout   = 5 * time.Minute
	qrcodeSessionRetain  = 10 * time.Minute
	qrcodeImageMaxBytes  = 2 << 20
	qrcodeImagePollDelay = 100 * time.Millisecond
)

var errQRCodeLoginActive = errors.New("a qrcode login is already in progress")

type qrcodeLoginView struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	Message    string     `json:"message"`
	Filename   string     `json:"filename"`
	Main       bool       `json:"main"`
	ImageReady bool       `json:"imageReady"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

type qrcodeLoginSession struct {
	id               string
	status           string
	message          string
	filename         string
	main             bool
	dir              string
	image            []byte
	createdAt        time.Time
	expiresAt        time.Time
	finishedAt       time.Time
	process          *managedProcess
	cancelRequested  bool
	expectedRevision string
}

type qrcodeCommandFactory func(context.Context, string, ...string) *exec.Cmd
type accountResultCommitter func(string, loginresult.Result) error

type qrcodeLoginManager struct {
	mu         sync.Mutex
	wg         sync.WaitGroup
	ctx        context.Context
	executable string
	configPath string
	home       string
	command    qrcodeCommandFactory
	supervisor *processSupervisor
	commit     accountResultCommitter
	sessions   map[string]*qrcodeLoginSession
	activeID   string
}

func newQRCodeLoginManager(ctx context.Context, executable, configPath, home string, commit accountResultCommitter, supervisors ...*processSupervisor) *qrcodeLoginManager {
	supervisor := newProcessSupervisor()
	if len(supervisors) > 0 && supervisors[0] != nil {
		supervisor = supervisors[0]
	}
	return &qrcodeLoginManager{
		ctx: ctx, executable: executable, configPath: configPath, home: home,
		command: exec.CommandContext, supervisor: supervisor, commit: commit, sessions: make(map[string]*qrcodeLoginSession),
	}
}

func (m *qrcodeLoginManager) start(filename string, main bool, expectedRevision string) (qrcodeLoginView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked(time.Now())
	if active := m.sessions[m.activeID]; active != nil && !qrcodeLoginTerminal(active.status) {
		return qrcodeLoginView{}, errQRCodeLoginActive
	}

	dir, err := os.MkdirTemp(m.home, ".qrcode-login-*")
	if err != nil {
		return qrcodeLoginView{}, err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return qrcodeLoginView{}, err
	}

	args := []string{
		"--config", m.configPath,
		"--home", m.home,
		"login", "qrcode",
		"--timeout", qrcodeLoginTimeout.String(),
		"--dir", dir,
		"--output", filename,
		"--no-config-write",
		"--json-result",
	}
	if main {
		args = append(args, "--main")
	}
	output := newTailBuffer(defaultProcessOutputLimit)
	process, err := m.supervisor.Start(m.ctx, processSpec{
		Kind: "qrcode-login", Command: m.executable, Args: args, Dir: m.home,
		Env: append(os.Environ(), "NCMM_WEB_CHILD=1"), Stdout: output, Stderr: output,
		Timeout: qrcodeLoginTimeout + 30*time.Second, CommandFactory: processCommandFactory(m.command),
	})
	if err != nil {
		_ = os.RemoveAll(dir)
		return qrcodeLoginView{}, fmt.Errorf("start qrcode login: %w", err)
	}

	now := time.Now()
	session := &qrcodeLoginSession{
		id: uuid.NewString(), status: "starting", message: "正在生成二维码",
		filename: filename, main: main, dir: dir, createdAt: now,
		expiresAt: now.Add(qrcodeLoginTimeout), process: process,
		expectedRevision: expectedRevision,
	}
	m.sessions[session.id] = session
	m.activeID = session.id
	m.wg.Add(1)
	go m.watch(session, process, output)
	return qrcodeLoginViewLocked(session), nil
}

func (m *qrcodeLoginManager) watch(session *qrcodeLoginSession, process *managedProcess, output *tailBuffer) {
	defer m.wg.Done()
	done := make(chan error, 1)
	go func() { done <- process.Wait() }()
	ticker := time.NewTicker(qrcodeImagePollDelay)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			m.captureImage(session)
			m.finish(session, process.ContextError(), strings.TrimSpace(string(output.Bytes())), err)
			_ = os.RemoveAll(session.dir)
			return
		case <-ticker.C:
			m.captureImage(session)
		}
	}
}

func (m *qrcodeLoginManager) captureImage(session *qrcodeLoginSession) {
	data, err := os.ReadFile(filepath.Join(session.dir, "qrcode.png"))
	if err != nil || len(data) > qrcodeImageMaxBytes || !isPNG(data) {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(session.image) != 0 {
		return
	}
	session.image = append([]byte(nil), data...)
	if session.status == "starting" {
		session.status = "waiting"
		session.message = "请使用网易云音乐 App 扫描二维码并确认登录"
	}
}

func (m *qrcodeLoginManager) finish(session *qrcodeLoginSession, contextErr error, output string, commandErr error) {
	if commandErr == nil && contextErr == nil {
		result, err := loginresult.Parse(output)
		if err == nil && result.Main != session.main {
			err = fmt.Errorf("login process returned an unexpected account type")
		}
		if err == nil && !strings.EqualFold(filepath.Base(result.CookiePath), session.filename) {
			err = fmt.Errorf("login process returned an unexpected Cookie path")
		}
		if err == nil && m.commit == nil {
			err = fmt.Errorf("account config committer is unavailable")
		}
		if err == nil {
			err = m.commit(session.expectedRevision, result)
		}
		if err != nil {
			commandErr = fmt.Errorf("Cookie 已保存，但账号配置提交失败: %w", err)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	session.finishedAt = now
	if m.activeID == session.id {
		m.activeID = ""
	}
	switch {
	case session.cancelRequested || errors.Is(contextErr, context.Canceled):
		session.status = "canceled"
		session.message = "二维码登录已取消"
	case errors.Is(contextErr, context.DeadlineExceeded) || strings.Contains(output, "context deadline exceeded"):
		session.status = "expired"
		session.message = "二维码登录已超时，请重新生成"
	case commandErr != nil:
		session.status = "failed"
		session.message = qrcodeCommandError(output, commandErr)
	default:
		session.status = "succeeded"
		session.message = fmt.Sprintf("登录成功，Cookie 已保存为 %s", session.filename)
	}
}

func (m *qrcodeLoginManager) get(id string) (qrcodeLoginView, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked(time.Now())
	session, ok := m.sessions[id]
	if !ok {
		return qrcodeLoginView{}, false
	}
	return qrcodeLoginViewLocked(session), true
}

func (m *qrcodeLoginManager) current() (qrcodeLoginView, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked(time.Now())
	session, ok := m.sessions[m.activeID]
	if !ok || qrcodeLoginTerminal(session.status) {
		return qrcodeLoginView{}, false
	}
	return qrcodeLoginViewLocked(session), true
}

func (m *qrcodeLoginManager) image(id string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked(time.Now())
	session, ok := m.sessions[id]
	if !ok || len(session.image) == 0 {
		return nil, false
	}
	return append([]byte(nil), session.image...), true
}

func (m *qrcodeLoginManager) cancelSession(id string) (qrcodeLoginView, bool) {
	m.mu.Lock()
	session, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return qrcodeLoginView{}, false
	}
	if qrcodeLoginTerminal(session.status) {
		view := qrcodeLoginViewLocked(session)
		m.mu.Unlock()
		return view, true
	}
	session.cancelRequested = true
	session.status = "cancelling"
	session.message = "正在取消二维码登录"
	process := session.process
	view := qrcodeLoginViewLocked(session)
	m.mu.Unlock()
	process.Stop()
	return view, true
}

func (m *qrcodeLoginManager) close() {
	m.mu.Lock()
	processes := make([]*managedProcess, 0, len(m.sessions))
	for _, session := range m.sessions {
		if !qrcodeLoginTerminal(session.status) {
			session.cancelRequested = true
			processes = append(processes, session.process)
		}
	}
	m.mu.Unlock()
	for _, process := range processes {
		process.Stop()
	}
	m.wg.Wait()
}

func (m *qrcodeLoginManager) pruneLocked(now time.Time) {
	for id, session := range m.sessions {
		if !session.finishedAt.IsZero() && now.Sub(session.finishedAt) > qrcodeSessionRetain {
			delete(m.sessions, id)
		}
	}
}

func qrcodeLoginViewLocked(session *qrcodeLoginSession) qrcodeLoginView {
	view := qrcodeLoginView{
		ID: session.id, Status: session.status, Message: session.message,
		Filename: session.filename, Main: session.main, ImageReady: len(session.image) != 0,
		CreatedAt: session.createdAt, ExpiresAt: session.expiresAt,
	}
	if !session.finishedAt.IsZero() {
		finishedAt := session.finishedAt
		view.FinishedAt = &finishedAt
	}
	return view
}

func qrcodeLoginTerminal(status string) bool {
	return status == "succeeded" || status == "failed" || status == "expired" || status == "canceled"
}

func qrcodeCommandError(output string, commandErr error) string {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "Error:") {
			message := strings.TrimSpace(strings.TrimPrefix(line, "Error:"))
			if message != "" {
				return message
			}
		}
	}
	if commandErr != nil {
		return commandErr.Error()
	}
	return "二维码登录失败"
}

func isPNG(data []byte) bool {
	return len(data) >= 8 && bytes.Equal(data[:8], []byte{'\x89', 'P', 'N', 'G', '\r', '\n', '\x1a', '\n'})
}
