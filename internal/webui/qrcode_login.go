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
	id              string
	status          string
	message         string
	filename        string
	main            bool
	dir             string
	image           []byte
	createdAt       time.Time
	expiresAt       time.Time
	finishedAt      time.Time
	cancel          context.CancelFunc
	cancelRequested bool
}

type qrcodeCommandFactory func(context.Context, string, ...string) *exec.Cmd

type qrcodeLoginManager struct {
	mu         sync.Mutex
	wg         sync.WaitGroup
	ctx        context.Context
	executable string
	configPath string
	home       string
	command    qrcodeCommandFactory
	sessions   map[string]*qrcodeLoginSession
	activeID   string
}

func newQRCodeLoginManager(ctx context.Context, executable, configPath, home string) *qrcodeLoginManager {
	return &qrcodeLoginManager{
		ctx: ctx, executable: executable, configPath: configPath, home: home,
		command: exec.CommandContext, sessions: make(map[string]*qrcodeLoginSession),
	}
}

func (m *qrcodeLoginManager) start(filename string, main bool) (qrcodeLoginView, error) {
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

	ctx, cancel := context.WithTimeout(m.ctx, qrcodeLoginTimeout+30*time.Second)
	args := []string{
		"--config", m.configPath,
		"--home", m.home,
		"login", "qrcode",
		"--timeout", qrcodeLoginTimeout.String(),
		"--dir", dir,
		"--output", filename,
	}
	if main {
		args = append(args, "--main")
	}
	cmd := m.command(ctx, m.executable, args...)
	cmd.Dir = m.home
	cmd.Env = append(os.Environ(), "NCMM_WEB_CHILD=1")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		cancel()
		_ = os.RemoveAll(dir)
		return qrcodeLoginView{}, fmt.Errorf("start qrcode login: %w", err)
	}

	now := time.Now()
	session := &qrcodeLoginSession{
		id: uuid.NewString(), status: "starting", message: "正在生成二维码",
		filename: filename, main: main, dir: dir, createdAt: now,
		expiresAt: now.Add(qrcodeLoginTimeout), cancel: cancel,
	}
	m.sessions[session.id] = session
	m.activeID = session.id
	m.wg.Add(1)
	go m.watch(session, ctx, cmd, &output)
	return qrcodeLoginViewLocked(session), nil
}

func (m *qrcodeLoginManager) watch(session *qrcodeLoginSession, ctx context.Context, cmd *exec.Cmd, output *bytes.Buffer) {
	defer m.wg.Done()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ticker := time.NewTicker(qrcodeImagePollDelay)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			m.captureImage(session)
			m.finish(session, ctx, strings.TrimSpace(output.String()), err)
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

func (m *qrcodeLoginManager) finish(session *qrcodeLoginSession, ctx context.Context, output string, commandErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	session.finishedAt = now
	if m.activeID == session.id {
		m.activeID = ""
	}
	switch {
	case session.cancelRequested || errors.Is(ctx.Err(), context.Canceled):
		session.status = "canceled"
		session.message = "二维码登录已取消"
	case errors.Is(ctx.Err(), context.DeadlineExceeded) || strings.Contains(output, "context deadline exceeded"):
		session.status = "expired"
		session.message = "二维码登录已超时，请重新生成"
	case commandErr != nil:
		session.status = "failed"
		session.message = qrcodeCommandError(output, commandErr)
	default:
		session.status = "succeeded"
		session.message = fmt.Sprintf("登录成功，Cookie 已保存为 %s", session.filename)
	}
	session.cancel()
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
	cancel := session.cancel
	view := qrcodeLoginViewLocked(session)
	m.mu.Unlock()
	cancel()
	return view, true
}

func (m *qrcodeLoginManager) close() {
	m.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(m.sessions))
	for _, session := range m.sessions {
		if !qrcodeLoginTerminal(session.status) {
			session.cancelRequested = true
			cancels = append(cancels, session.cancel)
		}
	}
	m.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
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
