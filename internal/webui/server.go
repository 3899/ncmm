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
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/3899/ncmm/internal/loginresult"
	webauth "github.com/3899/ncmm/internal/webui/auth"
	"github.com/3899/ncmm/pkg/notify"
	"gopkg.in/yaml.v3"
)

//go:embed static/*
var staticFiles embed.FS

var ErrRestartRequested = errors.New("WebUI restart requested")

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
	processes    *processSupervisor
	instance     instanceMetadata
	startedAt    time.Time
	updateMu     sync.Mutex
	http         *http.Server
	restartCh    chan struct{}
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
	webConfig, err := newWebConfigStore(opts.WebConfig, opts.SchedulerMigration)
	if err != nil {
		return nil, err
	}
	webSettings := webConfig.snapshot()
	processes := newProcessSupervisor()
	runner, err := newRunManager(opts.Executable, opts.ConfigPath, opts.Home, webSettings.Logs, webSettings.Concurrency, processes)
	if err != nil {
		return nil, err
	}
	scheduler, err := newScheduler(ctx, webConfig, runner)
	if err != nil {
		return nil, err
	}
	configRepository := newConfigStore(opts.ConfigPath)
	s := &Server{
		opts: opts, authManager: authManager, loginLimiter: newLoginRateLimiter(),
		config: configRepository, notify: notifyStore,
		webConfig: webConfig, runner: runner, scheduler: scheduler,
		processes: processes, startedAt: time.Now(),
		restartCh: make(chan struct{}, 1),
		qrcode: newQRCodeLoginManager(ctx, opts.Executable, opts.ConfigPath, opts.Home, func(expectedRevision string, result loginresult.Result) error {
			_, updateErr := configRepository.updateAccount(expectedRevision, result)
			return updateErr
		}, processes),
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
	defer s.processes.Close(10 * time.Second)
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
	s.instance = metadata
	s.startedAt = metadata.StartedAt
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
	case <-s.restartCh:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.http.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return ErrRestartRequested
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
	mux.HandleFunc("GET /api/v1/instance", s.handleInstanceGet)
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
	mux.HandleFunc("GET /api/v1/config/schema", s.handleConfigSchemaGet)
	mux.HandleFunc("GET /api/v1/config", s.handleConfigGet)
	mux.HandleFunc("PUT /api/v1/config", s.handleConfigPut)
	mux.HandleFunc("GET /api/v1/notify", s.handleNotifyGet)
	mux.HandleFunc("PUT /api/v1/notify", s.handleNotifyPut)
	mux.HandleFunc("POST /api/v1/notify/{channel}/test", s.handleNotifyTest)
	mux.HandleFunc("GET /api/v1/settings", s.handleSettingsGet)
	mux.HandleFunc("PUT /api/v1/settings", s.handleSettingsPut)
	mux.HandleFunc("GET /api/v1/schedules", s.handleSchedulesGet)
	mux.HandleFunc("POST /api/v1/schedules", s.handleSchedulesPost)
	mux.HandleFunc("PUT /api/v1/schedules/{id}", s.handleSchedulePut)
	mux.HandleFunc("DELETE /api/v1/schedules/{id}", s.handleScheduleDelete)
	mux.HandleFunc("POST /api/v1/schedules/{id}/pin", s.handleSchedulePin)
	mux.HandleFunc("POST /api/v1/schedules/{id}/run", s.handleScheduleRun)
	mux.HandleFunc("GET /api/v1/runs", s.handleRunsGet)
	mux.HandleFunc("GET /api/v1/play-stats", s.handlePlayStatsGet)
	mux.HandleFunc("GET /api/v1/runs/{id}/log", s.handleRunLog)
	mux.HandleFunc("POST /api/v1/runs/{id}/stop", s.handleRunStop)
	mux.HandleFunc("DELETE /api/v1/runs/{id}", s.handleRunDelete)
	mux.HandleFunc("POST /api/v1/logs/cleanup", s.handleLogsCleanup)
	mux.HandleFunc("POST /api/v1/logs/cleanup/advanced", s.handleLogsAdvancedCleanup)
	mux.HandleFunc("POST /api/v1/system/restart", s.handleSystemRestart)
	mux.HandleFunc("POST /api/v1/accounts/cookie", s.handleCookieImport)
	mux.HandleFunc("GET /api/v1/accounts/qrcode", s.handleQRCodeLoginCurrent)
	mux.HandleFunc("POST /api/v1/accounts/qrcode", s.handleQRCodeLoginStart)
	mux.HandleFunc("GET /api/v1/accounts/qrcode/{id}", s.handleQRCodeLoginGet)
	mux.HandleFunc("GET /api/v1/accounts/qrcode/{id}/image", s.handleQRCodeLoginImage)
	mux.HandleFunc("DELETE /api/v1/accounts/qrcode/{id}", s.handleQRCodeLoginCancel)

	staticFS, _ := fs.Sub(staticFiles, "static")
	fileServer := http.FileServer(http.FS(staticFS))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		if r.Method == http.MethodGet && frontendRoute(r.URL.Path) {
			data, err := fs.ReadFile(staticFS, "index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(data)
			return
		}
		fileServer.ServeHTTP(w, r)
	}))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob: https://*.music.126.net; connect-src 'self'; frame-ancestors 'none'")
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
			if !publicAuthRequest(r) && s.authManager.ProtectionEnabled() {
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

func frontendRoute(path string) bool {
	switch path {
	case "/", "/account", "/task", "/config", "/logs", "/system":
		return true
	default:
		return false
	}
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
	case "/api/v1/instance":
		return r.Method == http.MethodGet
	case "/api/v1/auth/setup", "/api/v1/auth/login", "/api/v1/auth/logout":
		return r.Method == http.MethodPost
	default:
		return false
	}
}

func (s *Server) handleInstanceGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"instanceId": s.instance.InstanceID,
		"version":    s.opts.Version,
	})
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

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	doc, _ := s.config.get()
	listenURL, webURL := s.webURLs(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"version": s.opts.Version, "commit": s.opts.Commit, "branch": s.opts.Branch, "buildTime": s.opts.BuildTime,
		"startedAt":       s.startedAt,
		"schedulerActive": s.scheduler.isActive(),
		"timezone":        s.scheduler.timezone(),
		"schedules":       len(s.scheduler.list()), "running": s.runner.runningCount(),
		"queued": s.runner.queuedCount(),
		"logs":   s.runner.stats(), "configRevision": doc.Revision,
		"listenUrl": listenURL, "webUrl": webURL,
		"paths": map[string]string{
			"executable": absolutePath(s.opts.Executable),
			"database":   s.databasePath(doc),
			"config":     absolutePath(s.opts.ConfigPath),
			"notify":     absolutePath(s.notify.path),
		},
	})
}

func (s *Server) webURLs(r *http.Request) (string, string) {
	listen := s.opts.Listen
	if s.instance.Listen != "" {
		listen = s.instance.Listen
	}
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		host, port = "127.0.0.1", "3899"
	}
	if host == "" {
		host = "0.0.0.0"
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	listenURL := scheme + "://" + net.JoinHostPort(host, port)
	webHost := firstLANIPv4()
	if webHost == "" {
		webHost = host
		if webHost == "0.0.0.0" || webHost == "::" {
			webHost = "127.0.0.1"
		}
	}
	return listenURL, scheme + "://" + net.JoinHostPort(webHost, port)
}

func firstLANIPv4() string {
	interfaces, _ := net.Interfaces()
	var fallback string
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := networkInterface.Addrs()
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil || ip.To4() == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			if ip.IsPrivate() {
				return ip.String()
			}
			if fallback == "" {
				fallback = ip.String()
			}
		}
	}
	return fallback
}

func absolutePath(path string) string {
	if path == "" {
		return ""
	}
	if absolute, err := filepath.Abs(path); err == nil {
		return filepath.Clean(absolute)
	}
	return filepath.Clean(path)
}

func (s *Server) databasePath(document configDocument) string {
	root, ok := document.Data.(map[string]any)
	if !ok {
		return ""
	}
	database, ok := root["database"].(map[string]any)
	if !ok {
		return ""
	}
	path, _ := database["path"].(string)
	path = strings.ReplaceAll(strings.TrimSpace(path), "${HOME}", s.opts.Home)
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(s.opts.ConfigPath), path)
	}
	return absolutePath(path)
}

func (s *Server) handleConfigGet(w http.ResponseWriter, _ *http.Request) {
	doc, err := s.config.get()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, withYAMLSections(doc))
}

func (s *Server) handleConfigSchemaGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, configurationSchema())
}

func (s *Server) handleConfigPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision string  `json:"revision"`
		Raw      *string `json:"raw"`
		Data     any     `json:"data"`
		Section  string  `json:"section"`
	}
	if !decodeJSON(w, r, &req, 4<<20) {
		return
	}
	var doc configDocument
	var err error
	if req.Raw != nil {
		if req.Section != "" {
			doc, err = s.config.saveSectionRaw(req.Revision, req.Section, *req.Raw)
		} else {
			doc, err = s.config.saveRaw(req.Revision, *req.Raw)
		}
	} else if req.Data != nil {
		doc, err = s.config.saveData(req.Revision, req.Data)
	} else {
		err = fmt.Errorf("raw or data is required")
	}
	if err != nil {
		writeError(w, configWriteStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, withYAMLSections(doc))
}

func (s *Server) handleNotifyGet(w http.ResponseWriter, _ *http.Request) {
	doc, err := s.notify.get()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, withYAMLSections(doc))
}

func (s *Server) handleNotifyPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision string  `json:"revision"`
		Raw      *string `json:"raw"`
		Data     any     `json:"data"`
		Section  string  `json:"section"`
	}
	if !decodeJSON(w, r, &req, 2<<20) {
		return
	}
	var doc configDocument
	var err error
	if req.Raw != nil {
		if req.Section != "" {
			if !validNotifyChannel(req.Section) {
				writeError(w, http.StatusBadRequest, "unknown notify channel")
				return
			}
			doc, err = s.notify.saveSectionRaw(req.Revision, req.Section, *req.Raw)
		} else {
			doc, err = s.notify.saveRaw(req.Revision, *req.Raw)
		}
	} else if req.Data != nil {
		doc, err = s.notify.saveData(req.Revision, req.Data)
	} else {
		err = fmt.Errorf("raw or data is required")
	}
	if err != nil {
		writeError(w, configWriteStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, withYAMLSections(doc))
}

func (s *Server) handleNotifyTest(w http.ResponseWriter, r *http.Request) {
	channel := r.PathValue("channel")
	name, ok := notifyChannelNames[channel]
	if !ok {
		writeError(w, http.StatusNotFound, "unknown notify channel")
		return
	}
	cfg, _, err := notify.LoadChannels(s.notify.path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	selected, err := selectNotifyChannel(cfg, channel)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	dispatcher := notify.NewDispatcher(selected, 10*time.Second)
	if dispatcher.Len() != 1 {
		writeError(w, http.StatusBadRequest, "当前通道配置不完整")
		return
	}
	message := notify.Message{
		Title:   name + " 信使打卡成功✅",
		Content: "服务支持：NCMM 开源项目\n简介：网易云音乐一站式任务管理系统\nGitHub：https://github.com/3899/ncmm",
		Level:   "info",
	}
	if err := dispatcher.SendAll(r.Context(), message); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": true, "channel": channel, "name": name})
}

var notifyChannelNames = map[string]string{
	"webhook": "Webhook", "bark": "Bark", "serverchan": "Server 酱",
	"telegram": "Telegram", "dingtalk": "钉钉", "coolpush": "CoolPush",
	"pushplus": "PushPlus", "wecom_key": "企业微信群", "wecom_app": "企业微信应用",
}

func validNotifyChannel(channel string) bool {
	_, ok := notifyChannelNames[channel]
	return ok
}

func selectNotifyChannel(cfg *notify.ChannelsConfig, channel string) (*notify.ChannelsConfig, error) {
	selected := &notify.ChannelsConfig{}
	switch channel {
	case "webhook":
		selected.Webhook = cfg.Webhook
		selected.Webhook.Enabled = true
	case "bark":
		selected.Bark = cfg.Bark
		selected.Bark.Enabled = true
	case "serverchan":
		selected.ServerChan = cfg.ServerChan
		selected.ServerChan.Enabled = true
	case "telegram":
		selected.Telegram = cfg.Telegram
		selected.Telegram.Enabled = true
	case "dingtalk":
		selected.DingTalk = cfg.DingTalk
		selected.DingTalk.Enabled = true
	case "coolpush":
		selected.CoolPush = cfg.CoolPush
		selected.CoolPush.Enabled = true
	case "pushplus":
		selected.PushPlus = cfg.PushPlus
		selected.PushPlus.Enabled = true
	case "wecom_key":
		selected.WeComKey = cfg.WeComKey
		selected.WeComKey.Enabled = true
	case "wecom_app":
		selected.WeComApp = cfg.WeComApp
		selected.WeComApp.Enabled = true
	default:
		return nil, fmt.Errorf("unknown notify channel %q", channel)
	}
	return selected, nil
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
		"timezone": cfg.Timezone, "logs": cfg.Logs, "concurrency": cfg.Concurrency,
		"stats": s.runner.stats(),
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

func (s *Server) handleSchedulePin(w http.ResponseWriter, r *http.Request) {
	if err := s.scheduler.pin(r.PathValue("id")); err != nil {
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

func (s *Server) handleRunDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.runner.deleteRun(r.PathValue("id")); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLogsCleanup(w http.ResponseWriter, _ *http.Request) {
	if err := s.runner.cleanup(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.runner.stats())
}

func (s *Server) handleLogsAdvancedCleanup(w http.ResponseWriter, r *http.Request) {
	var filter LogCleanupFilter
	if !decodeJSON(w, r, &filter, 32<<10) {
		return
	}
	result, err := s.runner.cleanupMatching(filter)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleSystemRestart(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusAccepted, map[string]bool{"restarting": true})
	go func() {
		time.Sleep(150 * time.Millisecond)
		select {
		case s.restartCh <- struct{}{}:
		default:
		}
	}()
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
	output, err := s.processes.RunOutput(r.Context(), processSpec{
		Kind: "cookie-login", Command: s.opts.Executable, Args: args, Dir: s.opts.Home,
		Env: append(os.Environ(), "NCMM_WEB_CHILD=1"), Timeout: 90 * time.Second,
	}, defaultProcessOutputLimit)
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
