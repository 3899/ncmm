package webui

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/3899/ncmm/internal/loginresult"
	webauth "github.com/3899/ncmm/internal/webui/auth"
	"gopkg.in/yaml.v3"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	opts         Options
	authManager  *webauth.Manager
	loginLimiter *loginRateLimiter
	config       *configStore
	notify       *configStore
	webConfig    *webConfigStore
	runner       *runManager
	scheduler    *scheduler
	qrcode       *qrcodeLoginManager
	updateMu     sync.Mutex
	http         *http.Server
}

func New(ctx context.Context, opts Options) (*Server, error) {
	if opts.Listen == "" {
		opts.Listen = defaultListen
	}
	if opts.Output == nil {
		opts.Output = func(string, ...any) {}
	}
	if opts.WebConfig == "" {
		opts.WebConfig = filepath.Join(opts.Home, "webui.yaml")
	}
	authManager, err := webauth.NewManager(filepath.Join(opts.Home, webauth.DefaultStoreName))
	if err != nil {
		return nil, fmt.Errorf("initialize WebUI authentication: %w", err)
	}
	notifyPath, err := resolveNotifyPath(opts.ConfigPath, opts.Home)
	if err != nil {
		return nil, err
	}
	notifyStore, err := newNotifyStore(notifyPath)
	if err != nil {
		return nil, fmt.Errorf("initialize notify store: %w", err)
	}
	webConfig, err := newWebConfigStore(opts.WebConfig)
	if err != nil {
		return nil, err
	}
	webSettings := webConfig.snapshot()
	runner, err := newRunManager(opts.Executable, opts.ConfigPath, opts.Home, webSettings.Logs, webSettings.Concurrency)
	if err != nil {
		return nil, err
	}
	scheduler, err := newScheduler(ctx, opts.Scheduler, webConfig, runner)
	if err != nil {
		return nil, err
	}
	configRepository := newConfigStore(opts.ConfigPath)
	s := &Server{
		opts: opts, authManager: authManager, loginLimiter: newLoginRateLimiter(),
		config: configRepository, notify: notifyStore,
		webConfig: webConfig, runner: runner, scheduler: scheduler,
		qrcode: newQRCodeLoginManager(ctx, opts.Executable, opts.ConfigPath, opts.Home, func(expectedRevision string, result loginresult.Result) error {
			_, updateErr := configRepository.updateAccount(expectedRevision, result)
			return updateErr
		}),
	}
	s.http = &http.Server{
		Addr:              opts.Listen,
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       60 * time.Second,
	}
	return s, nil
}

func (s *Server) Run(ctx context.Context) error {
	defer s.qrcode.close()
	instanceLock, err := acquireWebInstanceLock(s.opts.Home)
	if err != nil {
		return err
	}
	defer instanceLock.Close()
	listener, err := net.Listen("tcp", s.opts.Listen)
	if err != nil {
		return fmt.Errorf("bind WebUI listener %q: %w", s.opts.Listen, err)
	}
	defer listener.Close()
	listen := listener.Addr().String()
	metadata, err := instanceLock.writeMetadata(listen, s.opts.Version)
	if err != nil {
		return err
	}
	s.scheduler.start()
	defer s.scheduler.close()
	s.opts.Output("[webui] listening on http://%s (instance %s)\n", listen, metadata.InstanceID)
	go s.cleanupLoop(ctx)
	errCh := make(chan error, 1)
	go func() {
		serveErr := s.http.Serve(listener)
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		errCh <- serveErr
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.runner.cleanup()
		}
	}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/v1/auth/setup", s.handleAuthSetup)
	mux.HandleFunc("POST /api/v1/auth/login", s.handleAuthLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", s.handleAuthLogout)
	mux.HandleFunc("PUT /api/v1/auth/password", s.handleAuthPasswordPut)
	mux.HandleFunc("GET /api/v1/auth/settings", s.handleAuthSettingsGet)
	mux.HandleFunc("PUT /api/v1/auth/settings", s.handleAuthSettingsPut)
	mux.HandleFunc("GET /api/v1/auth/sessions", s.handleAuthSessionsGet)
	mux.HandleFunc("DELETE /api/v1/auth/sessions/{id}", s.handleAuthSessionDelete)
	mux.HandleFunc("POST /api/v1/auth/sessions/revoke-others", s.handleAuthSessionsRevokeOthers)
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/update", s.handleUpdateGet)
	mux.HandleFunc("POST /api/v1/update/check", s.handleUpdateCheck)
	mux.HandleFunc("POST /api/v1/update/apply", s.handleUpdateApply)
	mux.HandleFunc("GET /api/v1/config", s.handleConfigGet)
	mux.HandleFunc("PUT /api/v1/config", s.handleConfigPut)
	mux.HandleFunc("GET /api/v1/notify", s.handleNotifyGet)
	mux.HandleFunc("PUT /api/v1/notify", s.handleNotifyPut)
	mux.HandleFunc("GET /api/v1/settings", s.handleSettingsGet)
	mux.HandleFunc("PUT /api/v1/settings", s.handleSettingsPut)
	mux.HandleFunc("GET /api/v1/schedules", s.handleSchedulesGet)
	mux.HandleFunc("POST /api/v1/schedules", s.handleSchedulesPost)
	mux.HandleFunc("PUT /api/v1/schedules/{id}", s.handleSchedulePut)
	mux.HandleFunc("DELETE /api/v1/schedules/{id}", s.handleScheduleDelete)
	mux.HandleFunc("POST /api/v1/schedules/{id}/run", s.handleScheduleRun)
	mux.HandleFunc("GET /api/v1/runs", s.handleRunsGet)
	mux.HandleFunc("GET /api/v1/runs/{id}/log", s.handleRunLog)
	mux.HandleFunc("POST /api/v1/runs/{id}/stop", s.handleRunStop)
	mux.HandleFunc("POST /api/v1/logs/cleanup", s.handleLogsCleanup)
	mux.HandleFunc("POST /api/v1/accounts/cookie", s.handleCookieImport)
	mux.HandleFunc("GET /api/v1/accounts/qrcode", s.handleQRCodeLoginCurrent)
	mux.HandleFunc("POST /api/v1/accounts/qrcode", s.handleQRCodeLoginStart)
	mux.HandleFunc("GET /api/v1/accounts/qrcode/{id}", s.handleQRCodeLoginGet)
	mux.HandleFunc("GET /api/v1/accounts/qrcode/{id}/image", s.handleQRCodeLoginImage)
	mux.HandleFunc("DELETE /api/v1/accounts/qrcode/{id}", s.handleQRCodeLoginCancel)

	staticFS, _ := fs.Sub(staticFiles, "static")
	fileServer := http.FileServer(http.FS(staticFS))
	mux.Handle("/", fileServer)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; connect-src 'self'; frame-ancestors 'none'")
		listen := s.opts.Listen
		if listen == "" {
			listen = defaultListen
		}
		if !requestHostAllowed(listen, r.Host) {
			writeError(w, http.StatusMisdirectedRequest, "request Host is not allowed for this WebUI listener")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if isMutation(r.Method) && !mutationSourceAllowed(r) {
				writeError(w, http.StatusForbidden, "cross-origin request is not allowed")
				return
			}
			if !publicAuthRequest(r) {
				authenticated, ok := s.authenticateRequest(r)
				if !ok {
					writeError(w, http.StatusUnauthorized, "authentication required")
					return
				}
				if isMutation(r.Method) && !constantStringEqual(r.Header.Get(csrfHeader), authenticated.Session.CSRFToken) {
					writeError(w, http.StatusForbidden, "invalid CSRF token")
					return
				}
				r = r.WithContext(withRequestAuthentication(r.Context(), authenticated))
			}
		}
		mux.ServeHTTP(w, r)
	})
}

type requestAuth struct {
	Session webauth.AuthenticatedSession
}

type requestAuthKey struct{}

func (s *Server) authenticateRequest(r *http.Request) (requestAuth, bool) {
	if s.authManager != nil {
		if token := sessionToken(r); token != "" {
			session, err := s.authManager.Authenticate(r.Context(), token)
			if err == nil {
				return requestAuth{Session: session}, true
			}
		}
	}
	return requestAuth{}, false
}

func withRequestAuthentication(ctx context.Context, authenticated requestAuth) context.Context {
	return context.WithValue(ctx, requestAuthKey{}, authenticated)
}

func requestAuthentication(r *http.Request) requestAuth {
	authenticated, _ := r.Context().Value(requestAuthKey{}).(requestAuth)
	return authenticated
}

func publicAuthRequest(r *http.Request) bool {
	switch r.URL.Path {
	case "/api/v1/auth/status":
		return r.Method == http.MethodGet
	case "/api/v1/auth/setup", "/api/v1/auth/login", "/api/v1/auth/logout":
		return r.Method == http.MethodPost
	default:
		return false
	}
}

func isMutation(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete
}

func mutationSourceAllowed(r *http.Request) bool {
	if site := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))); site != "" && site != "same-origin" && site != "none" {
		return false
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && parsed.Scheme != "" && strings.EqualFold(parsed.Host, r.Host)
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	doc, _ := s.config.get()
	writeJSON(w, http.StatusOK, map[string]any{
		"version": s.opts.Version, "commit": s.opts.Commit, "buildTime": s.opts.BuildTime,
		"schedulerEnabled": s.scheduler.isEnabled(), "timezone": s.scheduler.timezone(),
		"schedules": len(s.scheduler.list()), "running": s.runner.runningCount(),
		"queued": s.runner.queuedCount(),
		"logs":   s.runner.stats(), "configRevision": doc.Revision,
	})
}

func (s *Server) handleConfigGet(w http.ResponseWriter, _ *http.Request) {
	doc, err := s.config.get()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (s *Server) handleConfigPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision string  `json:"revision"`
		Raw      *string `json:"raw"`
		Data     any     `json:"data"`
	}
	if !decodeJSON(w, r, &req, 4<<20) {
		return
	}
	var doc configDocument
	var err error
	if req.Raw != nil {
		doc, err = s.config.saveRaw(req.Revision, *req.Raw)
	} else if req.Data != nil {
		doc, err = s.config.saveData(req.Revision, req.Data)
	} else {
		err = fmt.Errorf("raw or data is required")
	}
	if err != nil {
		writeError(w, configWriteStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (s *Server) handleNotifyGet(w http.ResponseWriter, _ *http.Request) {
	doc, err := s.notify.get()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (s *Server) handleNotifyPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision string  `json:"revision"`
		Raw      *string `json:"raw"`
		Data     any     `json:"data"`
	}
	if !decodeJSON(w, r, &req, 2<<20) {
		return
	}
	var doc configDocument
	var err error
	if req.Raw != nil {
		doc, err = s.notify.saveRaw(req.Revision, *req.Raw)
	} else if req.Data != nil {
		doc, err = s.notify.saveData(req.Revision, req.Data)
	} else {
		err = fmt.Errorf("raw or data is required")
	}
	if err != nil {
		writeError(w, configWriteStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func resolveNotifyPath(configPath, home string) (string, error) {
	file := "notify.yaml"
	if configPath != "" && configPath != "default" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return "", err
		}
		var cfg struct {
			Notify struct {
				File string `yaml:"file"`
			} `yaml:"notify"`
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return "", fmt.Errorf("parse notify path from config: %w", err)
		}
		if strings.TrimSpace(cfg.Notify.File) != "" {
			file = strings.TrimSpace(cfg.Notify.File)
		}
	}
	if filepath.IsAbs(file) {
		return filepath.Clean(file), nil
	}
	base := home
	if configPath != "" && configPath != "default" {
		base = filepath.Dir(configPath)
	}
	if base == "" {
		base = "."
	}
	return filepath.Clean(filepath.Join(base, file)), nil
}

func (s *Server) handleSettingsGet(w http.ResponseWriter, _ *http.Request) {
	cfg := s.webConfig.snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"timezone": cfg.Timezone, "logs": cfg.Logs, "concurrency": cfg.Concurrency, "stats": s.runner.stats(),
	})
}

func (s *Server) handleSettingsPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Timezone    string            `json:"timezone"`
		Logs        LogPolicy         `json:"logs"`
		Concurrency ConcurrencyPolicy `json:"concurrency"`
	}
	if !decodeJSON(w, r, &req, 64<<10) {
		return
	}
	if _, err := time.LoadLocation(req.Timezone); err != nil {
		writeError(w, http.StatusBadRequest, "invalid timezone")
		return
	}
	cfg, err := s.webConfig.updateSettings(req.Timezone, req.Logs, req.Concurrency)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.runner.setPolicy(cfg.Logs)
	s.runner.setConcurrencyPolicy(cfg.Concurrency)
	if err := s.scheduler.reload(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.runner.cleanup()
	s.handleSettingsGet(w, r)
}

func (s *Server) handleSchedulesGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.scheduler.list())
}

func (s *Server) handleSchedulesPost(w http.ResponseWriter, r *http.Request) {
	var job Schedule
	if !decodeJSON(w, r, &job, 128<<10) {
		return
	}
	job.ID = ""
	saved, err := s.scheduler.upsert(job)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, saved)
}

func (s *Server) handleSchedulePut(w http.ResponseWriter, r *http.Request) {
	var job Schedule
	if !decodeJSON(w, r, &job, 128<<10) {
		return
	}
	job.ID = r.PathValue("id")
	saved, err := s.scheduler.upsert(job)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleScheduleDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.scheduler.delete(r.PathValue("id")); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleScheduleRun(w http.ResponseWriter, r *http.Request) {
	record, err := s.scheduler.runNow(r.PathValue("id"))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, record)
}

func (s *Server) handleRunsGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.runner.list())
}

func (s *Server) handleRunLog(w http.ResponseWriter, r *http.Request) {
	data, err := s.runner.readLog(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "run log not found")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func (s *Server) handleRunStop(w http.ResponseWriter, r *http.Request) {
	if err := s.runner.stop(r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, "running task not found")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"stopping": true})
}

func (s *Server) handleLogsCleanup(w http.ResponseWriter, _ *http.Request) {
	if err := s.runner.cleanup(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.runner.stats())
}

func (s *Server) handleCookieImport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content  string `json:"content"`
		Format   string `json:"format"`
		Filename string `json:"filename"`
		Main     bool   `json:"main"`
	}
	if !decodeJSON(w, r, &req, 2<<20) {
		return
	}
	if strings.TrimSpace(req.Filename) == "" {
		if req.Main {
			req.Filename = "cookie.json"
		} else {
			req.Filename = "fan1.json"
		}
	}
	filename, err := validateCookieFilename(req.Filename)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, "cookie content is required")
		return
	}
	if req.Format != "" && req.Format != "auto" && req.Format != "json" && req.Format != "netscape" && req.Format != "header" {
		writeError(w, http.StatusBadRequest, "unsupported cookie format")
		return
	}
	configDocument, err := s.config.get()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load config revision: "+err.Error())
		return
	}
	tmp, err := os.CreateTemp(s.opts.Home, ".cookie-import-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	_ = tmp.Chmod(0600)
	if _, err := io.WriteString(tmp, req.Content); err != nil {
		tmp.Close()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = tmp.Close()

	args := []string{
		"--config", s.opts.ConfigPath, "--home", s.opts.Home,
		"login", "cookie", "--file", tmpPath, "--output", filename,
		"--no-config-write", "--json-result",
	}
	if req.Format != "" && req.Format != "auto" {
		args = append(args, "--format", req.Format)
	}
	if req.Main {
		args = append(args, "--main")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.opts.Executable, args...)
	cmd.Dir = s.opts.Home
	cmd.Env = append(os.Environ(), "NCMM_WEB_CHILD=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		writeError(w, http.StatusBadRequest, message)
		return
	}
	result, err := loginresult.Parse(string(output))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if result.Main != req.Main || !strings.EqualFold(filepath.Base(result.CookiePath), filename) {
		writeError(w, http.StatusInternalServerError, "login process returned an unexpected account result")
		return
	}
	updated, err := s.config.updateAccount(configDocument.Revision, result)
	if err != nil {
		message := "Cookie 已保存，但账号配置提交失败: " + err.Error()
		writeError(w, configWriteStatus(err), message)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"filename": filename, "main": req.Main,
		"message": "账号登录配置已更新成功", "configRevision": updated.Revision,
	})
}

func (s *Server) handleQRCodeLoginStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Filename string `json:"filename"`
		Main     bool   `json:"main"`
	}
	if !decodeJSON(w, r, &req, 4<<10) {
		return
	}
	if strings.TrimSpace(req.Filename) == "" {
		if req.Main {
			req.Filename = "cookie.json"
		} else {
			req.Filename = "fan1.json"
		}
	}
	filename, err := validateCookieFilename(req.Filename)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.qrcode == nil {
		writeError(w, http.StatusServiceUnavailable, "qrcode login is unavailable")
		return
	}
	configDocument, err := s.config.get()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load config revision: "+err.Error())
		return
	}
	view, err := s.qrcode.start(filename, req.Main, configDocument.Revision)
	if errors.Is(err, errQRCodeLoginActive) {
		writeError(w, http.StatusConflict, "another qrcode login is already in progress")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, view)
}

func configWriteStatus(err error) int {
	switch {
	case errors.Is(err, errConfigRevisionRequired):
		return http.StatusPreconditionRequired
	case errors.Is(err, errConfigRevisionConflict):
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}

func (s *Server) handleQRCodeLoginCurrent(w http.ResponseWriter, _ *http.Request) {
	if s.qrcode == nil {
		writeError(w, http.StatusServiceUnavailable, "qrcode login is unavailable")
		return
	}
	view, ok := s.qrcode.current()
	if !ok {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleQRCodeLoginGet(w http.ResponseWriter, r *http.Request) {
	if s.qrcode == nil {
		writeError(w, http.StatusServiceUnavailable, "qrcode login is unavailable")
		return
	}
	view, ok := s.qrcode.get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "qrcode login session not found")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleQRCodeLoginImage(w http.ResponseWriter, r *http.Request) {
	if s.qrcode == nil {
		writeError(w, http.StatusServiceUnavailable, "qrcode login is unavailable")
		return
	}
	data, ok := s.qrcode.image(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "qrcode image is not ready")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleQRCodeLoginCancel(w http.ResponseWriter, r *http.Request) {
	if s.qrcode == nil {
		writeError(w, http.StatusServiceUnavailable, "qrcode login is unavailable")
		return
	}
	view, ok := s.qrcode.cancelSession(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "qrcode login session not found")
		return
	}
	writeJSON(w, http.StatusAccepted, view)
}

func validateCookieFilename(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("filename is required")
	}
	if filepath.Base(name) != name || strings.Contains(name, "..") || strings.ContainsAny(name, `/\\`) {
		return "", fmt.Errorf("filename must not contain a path")
	}
	if !strings.EqualFold(filepath.Ext(name), ".json") {
		name += ".json"
	}
	if len(name) > 128 {
		return "", fmt.Errorf("filename is too long")
	}
	return name, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
