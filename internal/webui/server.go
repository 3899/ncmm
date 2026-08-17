package webui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	opts              Options
	tokenMu           sync.RWMutex
	token             string
	tokenFromExternal bool
	config            *configStore
	notify            *configStore
	webConfig         *webConfigStore
	runner            *runManager
	scheduler         *scheduler
	updateMu          sync.Mutex
	http              *http.Server
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
	tokenFromExternal := strings.TrimSpace(opts.Token) != ""
	token, generated, err := resolveToken(opts.Home, opts.Token)
	if err != nil {
		return nil, err
	}
	if generated {
		opts.Output("[webui] generated management token: %s\n", token)
		opts.Output("[webui] token saved to: %s\n", filepath.Join(opts.Home, "webui.secret"))
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
	runner, err := newRunManager(opts.Executable, opts.ConfigPath, opts.Home, webConfig.snapshot().Logs)
	if err != nil {
		return nil, err
	}
	scheduler, err := newScheduler(ctx, opts.Scheduler, webConfig, runner)
	if err != nil {
		return nil, err
	}
	s := &Server{
		opts: opts, token: token, tokenFromExternal: tokenFromExternal,
		config: newConfigStore(opts.ConfigPath), notify: notifyStore,
		webConfig: webConfig, runner: runner, scheduler: scheduler,
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
	s.opts.Output("[webui] listening on http://%s\n", s.opts.Listen)
	go s.cleanupLoop(ctx)
	errCh := make(chan error, 1)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.scheduler.close()
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
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/security", s.handleSecurityGet)
	mux.HandleFunc("PUT /api/v1/security/token", s.handleSecurityTokenPut)
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

	staticFS, _ := fs.Sub(staticFiles, "static")
	fileServer := http.FileServer(http.FS(staticFS))
	mux.Handle("/", fileServer)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
		if strings.HasPrefix(r.URL.Path, "/api/") && !s.authorized(r) {
			writeError(w, http.StatusUnauthorized, "invalid management token")
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (s *Server) authorized(r *http.Request) bool {
	provided := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	s.tokenMu.RLock()
	defer s.tokenMu.RUnlock()
	if len(provided) != len(s.token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) == 1
}

func (s *Server) handleSecurityGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"externallyManaged": s.tokenFromExternal,
		"source":            map[bool]string{true: "启动参数或环境变量", false: "webui.secret"}[s.tokenFromExternal],
	})
}

func (s *Server) handleSecurityTokenPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &req, 4<<10) {
		return
	}
	token := strings.TrimSpace(req.Token)
	if err := validateManagementToken(token); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := os.MkdirAll(s.opts.Home, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := writeFileAtomic(filepath.Join(s.opts.Home, "webui.secret"), []byte(token+"\n"), 0600); err != nil {
		writeError(w, http.StatusInternalServerError, "save management token: "+err.Error())
		return
	}
	s.tokenMu.Lock()
	s.token = token
	s.tokenMu.Unlock()
	response := map[string]any{"updated": true, "externallyManaged": s.tokenFromExternal}
	if s.tokenFromExternal {
		response["warning"] = "当前令牌由 --token 或 NCMM_WEB_TOKEN 提供；本次运行已生效，但重启后仍会使用外部令牌"
	}
	writeJSON(w, http.StatusOK, response)
}

func validateManagementToken(token string) error {
	if len(token) < 8 {
		return fmt.Errorf("management token must be at least 8 characters")
	}
	if len(token) > 256 {
		return fmt.Errorf("management token is too long")
	}
	if strings.ContainsAny(token, "\r\n") {
		return fmt.Errorf("management token must be a single line")
	}
	return nil
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	doc, _ := s.config.get()
	writeJSON(w, http.StatusOK, map[string]any{
		"version": s.opts.Version, "commit": s.opts.Commit, "buildTime": s.opts.BuildTime,
		"schedulerEnabled": s.scheduler.isEnabled(), "timezone": s.scheduler.timezone(),
		"schedules": len(s.scheduler.list()), "running": s.runner.runningCount(),
		"logs": s.runner.stats(), "configRevision": doc.Revision,
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
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "changed since") {
			status = http.StatusConflict
		}
		writeError(w, status, err.Error())
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
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "changed since") {
			status = http.StatusConflict
		}
		writeError(w, status, err.Error())
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
	writeJSON(w, http.StatusOK, map[string]any{"timezone": cfg.Timezone, "logs": cfg.Logs, "stats": s.runner.stats()})
}

func (s *Server) handleSettingsPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Timezone string    `json:"timezone"`
		Logs     LogPolicy `json:"logs"`
	}
	if !decodeJSON(w, r, &req, 64<<10) {
		return
	}
	if _, err := time.LoadLocation(req.Timezone); err != nil {
		writeError(w, http.StatusBadRequest, "invalid timezone")
		return
	}
	cfg, err := s.webConfig.updateSettings(req.Timezone, req.Logs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.runner.setPolicy(cfg.Logs)
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

	args := []string{"--config", s.opts.ConfigPath, "--home", s.opts.Home, "login", "cookie", "--file", tmpPath, "--output", filename}
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
	writeJSON(w, http.StatusOK, map[string]any{
		"filename": filename, "main": req.Main, "message": strings.TrimSpace(string(output)),
	})
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

func resolveToken(home, configured string) (string, bool, error) {
	if strings.TrimSpace(configured) != "" {
		return strings.TrimSpace(configured), false, nil
	}
	path := filepath.Join(home, "webui.secret")
	if data, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(data)) != "" {
		return strings.TrimSpace(string(data)), false, nil
	}
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", false, err
	}
	token := hex.EncodeToString(raw[:])
	if err := os.MkdirAll(home, 0755); err != nil {
		return "", false, err
	}
	if err := os.WriteFile(path, []byte(token+"\n"), 0600); err != nil {
		return "", false, err
	}
	return token, true, nil
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
