package webui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateCookieFilename(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{name: "account", want: "account.json"},
		{name: "cookie.JSON", want: "cookie.JSON"},
		{name: "../cookie.json", wantErr: true},
		{name: `dir\cookie.json`, wantErr: true},
		{name: "", wantErr: true},
	}
	for _, test := range tests {
		got, err := validateCookieFilename(test.name)
		if test.wantErr {
			if err == nil {
				t.Fatalf("validateCookieFilename(%q) expected error", test.name)
			}
			continue
		}
		if err != nil || got != test.want {
			t.Fatalf("validateCookieFilename(%q) = %q, %v; want %q", test.name, got, err, test.want)
		}
	}
}

func TestValidateManagementToken(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{name: "valid", token: "new-token-123"},
		{name: "too short", token: "1234567", wantErr: true},
		{name: "multiline", token: "12345678\nsecond", wantErr: true},
		{name: "too long", token: strings.Repeat("x", 257), wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateManagementToken(test.token)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateManagementToken() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestManagementTokenUpdateSwitchesAuthorization(t *testing.T) {
	home := t.TempDir()
	server := &Server{opts: Options{Home: home}, token: "old-token-123"}
	handler := server.routes()

	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}

	response := request(http.MethodPut, "/api/v1/security/token", "old-token-123", `{"token":"new-token-456"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("token update status = %d, body = %s", response.Code, response.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(home, "webui.secret"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(data)); got != "new-token-456" {
		t.Fatalf("saved token = %q", got)
	}
	if got := request(http.MethodGet, "/api/v1/security", "old-token-123", "").Code; got != http.StatusUnauthorized {
		t.Fatalf("old token status = %d; want %d", got, http.StatusUnauthorized)
	}
	if got := request(http.MethodGet, "/api/v1/security", "new-token-456", "").Code; got != http.StatusOK {
		t.Fatalf("new token status = %d; want %d", got, http.StatusOK)
	}
}

func TestInitialSetupConfiguresManagementToken(t *testing.T) {
	home := t.TempDir()
	server := &Server{opts: Options{Home: home}, setupRequired: true}
	handler := server.routes()

	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", "http://example.com")
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}

	if got := request(http.MethodGet, "/api/v1/setup", "", "").Code; got != http.StatusOK {
		t.Fatalf("setup status = %d; want %d", got, http.StatusOK)
	}
	if got := request(http.MethodGet, "/api/v1/security", "", "").Code; got != http.StatusUnauthorized {
		t.Fatalf("protected API before setup = %d; want %d", got, http.StatusUnauthorized)
	}
	if got := request(http.MethodPost, "/api/v1/setup", "", `{"token":"new-token-123","confirmation":"different-token"}`).Code; got != http.StatusBadRequest {
		t.Fatalf("mismatched setup = %d; want %d", got, http.StatusBadRequest)
	}

	response := request(http.MethodPost, "/api/v1/setup", "", `{"token":"new-token-123","confirmation":"new-token-123"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("setup status = %d, body = %s", response.Code, response.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(home, "webui.secret"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(data)); got != "new-token-123" {
		t.Fatalf("saved setup token = %q", got)
	}
	if got := request(http.MethodGet, "/api/v1/security", "new-token-123", "").Code; got != http.StatusOK {
		t.Fatalf("new setup token status = %d; want %d", got, http.StatusOK)
	}
	if got := request(http.MethodPost, "/api/v1/setup", "", `{"token":"another-token","confirmation":"another-token"}`).Code; got != http.StatusConflict {
		t.Fatalf("repeated setup = %d; want %d", got, http.StatusConflict)
	}
}

func TestInitialSetupRejectsCrossSiteRequests(t *testing.T) {
	server := &Server{opts: Options{Home: t.TempDir()}, setupRequired: true}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup", strings.NewReader(`{"token":"new-token-123","confirmation":"new-token-123"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, req)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-site setup status = %d; want %d", response.Code, http.StatusForbidden)
	}
}

func TestResolveTokenRequiresInitialSetup(t *testing.T) {
	home := t.TempDir()
	token, required, err := resolveToken(home, "")
	if err != nil || token != "" || !required {
		t.Fatalf("resolveToken() = %q, %v, %v; want empty token and setup required", token, required, err)
	}
	if _, err := os.Stat(filepath.Join(home, "webui.secret")); !os.IsNotExist(err) {
		t.Fatalf("webui.secret should not be generated, stat error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(home, "webui.secret"), []byte("saved-token-123\n"), 0600); err != nil {
		t.Fatal(err)
	}
	token, required, err = resolveToken(home, "")
	if err != nil || token != "saved-token-123" || required {
		t.Fatalf("saved resolveToken() = %q, %v, %v", token, required, err)
	}
	token, required, err = resolveToken(home, "external-token-123")
	if err != nil || token != "external-token-123" || required {
		t.Fatalf("external resolveToken() = %q, %v, %v", token, required, err)
	}
}

func TestMakeUpdateViewUsesRunningVersion(t *testing.T) {
	t.Setenv("NCMM_DOCKER_OFFICIAL", "")
	server := &Server{opts: Options{Version: "1.2.0"}}
	view := server.makeUpdateView(updateState{
		CurrentVersion: "1.1.0",
		LatestVersion:  "1.2.0",
		UpdatedVersion: "1.2.0",
	}, false)
	if view.CurrentVersion != "1.2.0" || view.Available || view.RestartRequired {
		t.Fatalf("unexpected post-restart update view: %+v", view)
	}
}

func TestMakeUpdateViewPendingRestartAndDocker(t *testing.T) {
	t.Run("pending restart", func(t *testing.T) {
		t.Setenv("NCMM_DOCKER_OFFICIAL", "")
		server := &Server{opts: Options{Version: "1.1.0"}}
		view := server.makeUpdateView(updateState{LatestVersion: "1.2.0", UpdatedVersion: "1.2.0"}, false)
		if !view.Available || !view.RestartRequired || view.CanApply {
			t.Fatalf("unexpected pending-restart update view: %+v", view)
		}
	})

	t.Run("official docker", func(t *testing.T) {
		t.Setenv("NCMM_DOCKER_OFFICIAL", "1")
		server := &Server{opts: Options{Version: "1.1.0"}}
		view := server.makeUpdateView(updateState{LatestVersion: "1.2.0"}, false)
		if !view.Available || !view.Docker || view.CanApply {
			t.Fatalf("unexpected Docker update view: %+v", view)
		}
	})
}

func TestWebConfigStoreDefaultsAndUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.yaml")
	store, err := newWebConfigStore(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := store.snapshot()
	if cfg.Logs.RetentionDays != defaultRetentionDays || cfg.Timezone != defaultTimezone {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	job, err := store.upsert(Schedule{Name: "Daily", Enabled: true, Cron: "30 8 * * *", Command: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if job.ID == "" || len(store.snapshot().Jobs) != 1 {
		t.Fatalf("schedule was not persisted: %+v", job)
	}
	if _, err := store.updateSettings("UTC", LogPolicy{RetentionDays: 14, MaxTotalSizeMB: 128}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := newWebConfigStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.snapshot(); got.Timezone != "UTC" || got.Logs.RetentionDays != 14 || len(got.Jobs) != 1 {
		t.Fatalf("unexpected reloaded config: %+v", got)
	}
}

func TestConfigStorePreservesComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	raw := string(configForTest())
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	store := newConfigStore(path)
	doc, err := store.get()
	if err != nil {
		t.Fatal(err)
	}
	data := doc.Data.(map[string]any)
	data["version"] = "1.1.14"
	updated, err := store.saveData(doc.Revision, data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(updated.Raw, "# keep this comment") {
		t.Fatalf("comment was removed:\n%s", updated.Raw)
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
}

func TestConfigStoreReturnsFieldDescriptions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	raw := strings.Replace(
		string(configForTest()),
		"accounts:\n  main: \"./cookie.json\"",
		"accounts:\n  # 音乐人主账号 Cookie 文件路径\n  main: \"./cookie.json\" # 当前主账号\n  # 辅助账号 Cookie 列表",
		1,
	)
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := newConfigStore(path).get()
	if err != nil {
		t.Fatal(err)
	}
	description := doc.Descriptions["accounts.main"]
	if !strings.Contains(description, "音乐人主账号") || !strings.Contains(description, "当前主账号") {
		t.Fatalf("unexpected accounts.main description: %q", description)
	}
	if description := doc.Descriptions["accounts.secondary"]; description != "辅助账号 Cookie 列表" {
		t.Fatalf("unexpected accounts.secondary description: %q", description)
	}
}

func TestNotifyStoreInitializesAndSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notify.yaml")
	store, err := newNotifyStore(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := store.get()
	if err != nil {
		t.Fatal(err)
	}
	data := doc.Data.(map[string]any)
	webhook := data["webhook"].(map[string]any)
	if webhook["method"] != "POST" {
		t.Fatalf("unexpected default webhook method: %#v", webhook["method"])
	}
	pushplus := data["pushplus"].(map[string]any)
	pushplus["enabled"] = true
	pushplus["token"] = "test-token"
	updated, err := store.saveData(doc.Revision, data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(updated.Raw, "test-token") {
		t.Fatalf("notify value was not saved:\n%s", updated.Raw)
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("notify backup missing: %v", err)
	}
}

func TestResolveNotifyPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("notify:\n  file: channels/custom.yaml\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := resolveNotifyPath(configPath, filepath.Join(dir, "home"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "channels", "custom.yaml")
	if got != want {
		t.Fatalf("resolveNotifyPath() = %q; want %q", got, want)
	}
}

func TestRunCleanupHonorsRetention(t *testing.T) {
	dir := t.TempDir()
	m := &runManager{
		logDir:  dir,
		runs:    make(map[string]RunRecord),
		running: make(map[string]*runningProcess),
		jobRuns: make(map[string]string),
		policy:  LogPolicy{RetentionDays: 7, MaxTotalSizeMB: 16},
	}
	oldPath := filepath.Join(dir, "old.log")
	if err := os.WriteFile(oldPath, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().AddDate(0, 0, -8)
	if err := os.Chtimes(oldPath, old, old); err != nil {
		t.Fatal(err)
	}
	if err := m.cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expired log still exists: %v", err)
	}
}

func configForTest() []byte {
	return []byte(`# keep this comment
version: 1.1.14
accounts:
  main: "./cookie.json"
  secondary: []
network:
  debug: false
  timeout: 60s
  retry: 3
  user_agent:
    default: "test"
    weapi: "test"
    eapi: "test"
    xeapi: ""
  cookie:
    filepath: "./cookie.json"
    interval: 0s
log:
  app: test
  format: text
  level: info
  stdout: false
  rotate:
    filename: "./log/info.log"
    maxsize: 10
    maxage: 7
    maxbackups: 2
    localtime: true
    compress: true
database:
  driver: badger
  path: "./database"
`)
}
