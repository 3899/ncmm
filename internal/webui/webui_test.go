package webui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/3899/ncmm/internal/loginresult"
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

func TestInitialSetupConfiguresAdministratorPasswordAndSession(t *testing.T) {
	home := t.TempDir()
	server := newAuthTestServer(t, home)
	handler := server.routes()

	request := func(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", "http://localhost:3899")
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		req.Host = "localhost:3899"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}

	if got := request(http.MethodGet, "/api/v1/auth/status", "", nil).Code; got != http.StatusOK {
		t.Fatalf("auth status = %d; want %d", got, http.StatusOK)
	}
	if got := request(http.MethodGet, "/api/v1/auth/settings", "", nil).Code; got != http.StatusUnauthorized {
		t.Fatalf("protected API before setup = %d; want %d", got, http.StatusUnauthorized)
	}
	if got := request(http.MethodPost, "/api/v1/auth/setup", `{"password":"Admin 中文123#"}`, nil).Code; got != http.StatusBadRequest {
		t.Fatalf("invalid setup password = %d; want %d", got, http.StatusBadRequest)
	}

	response := request(http.MethodPost, "/api/v1/auth/setup", `{"password":"Admin#123"}`, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("setup status = %d, body = %s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "ncmm_session" || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("unexpected session cookie: %+v", cookies)
	}
	data, err := os.ReadFile(filepath.Join(home, "webui-auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Admin#123") || strings.Contains(string(data), cookies[0].Value) {
		t.Fatal("authentication store contains a plaintext credential")
	}
	if got := request(http.MethodGet, "/api/v1/auth/settings", "", cookies[0]).Code; got != http.StatusOK {
		t.Fatalf("new session status = %d; want %d", got, http.StatusOK)
	}
	if got := request(http.MethodPost, "/api/v1/auth/setup", `{"password":"Another#456"}`, nil).Code; got != http.StatusConflict {
		t.Fatalf("repeated setup = %d; want %d", got, http.StatusConflict)
	}
}

func TestInitialSetupRejectsCrossSiteRequests(t *testing.T) {
	server := newAuthTestServer(t, t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", strings.NewReader(`{"password":"Admin#123"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Host = "localhost:3899"
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, req)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-site setup status = %d; want %d", response.Code, http.StatusForbidden)
	}
}

func TestInitialSetupRejectsMismatchedOrigin(t *testing.T) {
	server := newAuthTestServer(t, t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", strings.NewReader(`{"password":"Admin#123"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://attacker.example:3899")
	req.Host = "localhost:3899"
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, req)
	if response.Code != http.StatusForbidden {
		t.Fatalf("mismatched Origin status = %d; want %d", response.Code, http.StatusForbidden)
	}
}

func TestWebUIRejectsUnexpectedHost(t *testing.T) {
	server := newAuthTestServer(t, t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	req.Host = "attacker.example:3899"
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, req)
	if response.Code != http.StatusMisdirectedRequest {
		t.Fatalf("unexpected Host status = %d; want %d", response.Code, http.StatusMisdirectedRequest)
	}
}

func TestInitialSetupAllowsOnlyOneConcurrentWinner(t *testing.T) {
	server := newAuthTestServer(t, t.TempDir())
	handler := server.routes()
	const attempts = 4
	start := make(chan struct{})
	var wg sync.WaitGroup
	var succeeded atomic.Int32
	var conflicted atomic.Int32
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", strings.NewReader(`{"password":"Admin#123"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", "http://localhost:3899")
			req.Host = "localhost:3899"
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			switch response.Code {
			case http.StatusOK:
				succeeded.Add(1)
			case http.StatusConflict:
				conflicted.Add(1)
			default:
				t.Errorf("concurrent setup status = %d, body = %s", response.Code, response.Body.String())
			}
		}()
	}
	close(start)
	wg.Wait()
	if succeeded.Load() != 1 || conflicted.Load() != attempts-1 {
		t.Fatalf("concurrent setup results: success=%d conflict=%d", succeeded.Load(), conflicted.Load())
	}
}

func TestNewAllowsUnconfiguredRemoteListener(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server, err := New(ctx, Options{Home: t.TempDir(), Listen: "0.0.0.0:3899"})
	if err != nil {
		t.Fatal(err)
	}
	defer server.scheduler.close()
	defer server.qrcode.close()
	configured, err := server.authManager.Configured(ctx)
	if err != nil || configured {
		t.Fatalf("new remote authentication state = configured %v, err %v", configured, err)
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
	if cfg.Version != defaultWebConfigVersion || cfg.Logs.RetentionDays != defaultRetentionDays || cfg.Timezone != defaultTimezone || cfg.Concurrency.MaxParallel != 1 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	job, err := store.upsert(Schedule{Name: "Daily", Enabled: true, Cron: "30 8 * * *", Command: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if job.ID == "" || len(store.snapshot().Jobs) != 1 {
		t.Fatalf("schedule was not persisted: %+v", job)
	}
	if _, err := store.updateSettings("UTC", LogPolicy{RetentionDays: 14, MaxTotalSizeMB: 128}, ConcurrencyPolicy{MaxParallel: 2}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := newWebConfigStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.snapshot(); got.Timezone != "UTC" || got.Logs.RetentionDays != 14 || got.Concurrency.MaxParallel != 2 || len(got.Jobs) != 1 {
		t.Fatalf("unexpected reloaded config: %+v", got)
	}
}

func TestWebConfigStoreMigratesV1SchedulerSafely(t *testing.T) {
	legacy := []byte("version: 1\ntimezone: Asia/Shanghai\nlogs:\n  retentionDays: 7\n  maxTotalSizeMB: 256\nconcurrency:\n  maxParallel: 1\njobs:\n  - id: daily\n    name: Daily\n    enabled: true\n    cron: '30 8 * * *'\n    command: task\n")
	tests := []struct {
		name      string
		migration *SchedulerMigration
		want      bool
	}{
		{name: "safe disable", want: false},
		{name: "explicitly disabled", migration: &SchedulerMigration{PreserveEnabled: false}, want: false},
		{name: "legacy cli enabled", migration: &SchedulerMigration{PreserveEnabled: true}, want: true},
		{name: "managed entrypoint enabled", migration: &SchedulerMigration{PreserveEnabled: true}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "webui.yaml")
			if err := os.WriteFile(path, legacy, 0600); err != nil {
				t.Fatal(err)
			}
			store, err := newWebConfigStore(path, test.migration)
			if err != nil {
				t.Fatal(err)
			}
			cfg := store.snapshot()
			if cfg.Version != 2 || len(cfg.Jobs) != 1 || cfg.Jobs[0].Enabled != test.want {
				t.Fatalf("unexpected migrated config: %+v", cfg)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "scheduler:") {
				t.Fatalf("deprecated scheduler field was persisted:\n%s", data)
			}
			reloaded, err := newWebConfigStore(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := reloaded.snapshot(); got.Version != 2 || len(got.Jobs) != 1 || got.Jobs[0].Enabled != test.want {
				t.Fatalf("migration was not persisted: %+v", got)
			}
		})
	}
}

func TestWebConfigStoreMigratesDeprecatedV2SchedulerField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.yaml")
	legacy := []byte("version: 2\nscheduler:\n  enabled: false\n  migrationSource: v1-safe-paused\ntimezone: Asia/Shanghai\nlogs:\n  retentionDays: 7\n  maxTotalSizeMB: 256\nconcurrency:\n  maxParallel: 1\njobs:\n  - id: daily\n    name: Daily\n    enabled: true\n    cron: '30 8 * * *'\n    command: task\n")
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := newWebConfigStore(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := store.snapshot()
	if cfg.Version != 2 || len(cfg.Jobs) != 1 || cfg.Jobs[0].Enabled {
		t.Fatalf("deprecated v2 config data changed: %+v", cfg)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "scheduler:") {
		t.Fatalf("deprecated scheduler field was not removed:\n%s", data)
	}
}

func TestWebConfigStorePreservesEnabledJobsWhenDeprecatedV2SchedulerWasEnabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.yaml")
	legacy := []byte("version: 2\nscheduler:\n  enabled: true\ntimezone: Asia/Shanghai\nlogs:\n  retentionDays: 7\n  maxTotalSizeMB: 256\nconcurrency:\n  maxParallel: 1\njobs:\n  - id: daily\n    name: Daily\n    enabled: true\n    cron: '30 8 * * *'\n    command: task\n")
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := newWebConfigStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg := store.snapshot(); len(cfg.Jobs) != 1 || !cfg.Jobs[0].Enabled {
		t.Fatalf("enabled deprecated v2 config was not preserved: %+v", cfg)
	}
}

func TestWebConfigStoreWriteFailureDoesNotChangeMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.yaml")
	store, err := newWebConfigStore(path)
	if err != nil {
		t.Fatal(err)
	}
	existing, err := store.upsert(Schedule{Name: "Daily", Enabled: true, Cron: "30 8 * * *", Command: "task"})
	if err != nil {
		t.Fatal(err)
	}
	want := store.snapshot()
	wantFile, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeErr := errors.New("injected write failure")
	store.write = func(string, []byte, os.FileMode) error { return writeErr }

	operations := []struct {
		name string
		run  func() error
	}{
		{name: "settings", run: func() error {
			_, err := store.updateSettings("UTC", LogPolicy{RetentionDays: 14, MaxTotalSizeMB: 128}, ConcurrencyPolicy{MaxParallel: 2})
			return err
		}},
		{name: "update schedule", run: func() error {
			existing.Name = "Changed"
			_, err := store.upsert(existing)
			return err
		}},
		{name: "insert schedule", run: func() error {
			_, err := store.upsert(Schedule{Name: "Extra", Cron: "0 9 * * *", Command: "sign"})
			return err
		}},
		{name: "delete schedule", run: func() error { return store.delete(existing.ID) }},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			if err := operation.run(); !errors.Is(err, writeErr) {
				t.Fatalf("operation error = %v; want injected failure", err)
			}
			if got := store.snapshot(); !reflect.DeepEqual(got, want) {
				t.Fatalf("memory changed after failed write:\n got: %+v\nwant: %+v", got, want)
			}
			gotFile, err := os.ReadFile(path)
			if err != nil || !reflect.DeepEqual(gotFile, wantFile) {
				t.Fatalf("file changed after failed write: %q, %v", gotFile, err)
			}
		})
	}

	store.write = writeFileAtomic
	updated, err := store.updateSettings("UTC", LogPolicy{RetentionDays: 14, MaxTotalSizeMB: 128}, ConcurrencyPolicy{MaxParallel: 2})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Timezone != "UTC" || len(updated.Jobs) != 1 || updated.Jobs[0].Name != "Daily" {
		t.Fatalf("successful update retained a failed mutation: %+v", updated)
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

func TestConfigStoreReturnsRawInvalidYAMLForRepair(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notify.yaml")
	raw := "notify:\n  enabled: [invalid\n"
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	store := newYAMLStore(path, validateNotifyFile)
	document, err := store.get()
	if err != nil {
		t.Fatal(err)
	}
	if document.Raw != raw || document.Revision == "" || document.ParseError == "" || document.Data != nil {
		t.Fatalf("unexpected repair document: %+v", document)
	}
	fixed := "notify:\n  enabled: false\n"
	repaired, err := store.saveRaw(document.Revision, fixed)
	if err != nil {
		t.Fatal(err)
	}
	if repaired.ParseError != "" || repaired.Data == nil || repaired.Raw != fixed {
		t.Fatalf("unexpected repaired document: %+v", repaired)
	}
}

func TestConfigStoreRequiresRevisionAndRejectsConcurrentUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, configForTest(), 0600); err != nil {
		t.Fatal(err)
	}
	store := newConfigStore(path)
	document, err := store.get()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.saveRaw("", document.Raw); !errors.Is(err, errConfigRevisionRequired) {
		t.Fatalf("saveRaw() error = %v; want revision required", err)
	}

	rawA := strings.Replace(document.Raw, "timeout: 60s", "timeout: 61s", 1)
	rawB := strings.Replace(document.Raw, "timeout: 60s", "timeout: 62s", 1)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, raw := range []string{rawA, rawB} {
		raw := raw
		go func() {
			<-start
			_, saveErr := store.saveRaw(document.Revision, raw)
			results <- saveErr
		}()
	}
	close(start)
	var succeeded, conflicted int
	for i := 0; i < 2; i++ {
		switch err := <-results; {
		case err == nil:
			succeeded++
		case errors.Is(err, errConfigRevisionConflict):
			conflicted++
		default:
			t.Fatalf("unexpected concurrent save error: %v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent saves: succeeded=%d conflicted=%d", succeeded, conflicted)
	}
}

func TestConfigAPIRequiresRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, configForTest(), 0600); err != nil {
		t.Fatal(err)
	}
	server := newAuthTestServer(t, t.TempDir())
	credentials, err := server.authManager.Setup(context.Background(), "Admin#123", requestClientInfo(httptest.NewRequest(http.MethodGet, "/", nil)))
	if err != nil {
		t.Fatal(err)
	}
	server.config = newConfigStore(path)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config", strings.NewReader(`{"data":{"version":"1.2.0"}}`))
	req.Host = "localhost:3899"
	req.Header.Set(csrfHeader, credentials.Session.CSRFToken)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "ncmm_session", Value: credentials.Token})
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, req)
	if response.Code != http.StatusPreconditionRequired {
		t.Fatalf("missing revision status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestConfigStoreAccountUpdatePreservesOtherData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := strings.Replace(string(configForTest()), "accounts:\n", "accounts:\n  # existing main account\n", 1)
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	store := newConfigStore(path)
	document, err := store.get()
	if err != nil {
		t.Fatal(err)
	}
	updated, err := store.updateAccount(document.Revision, loginresult.Result{
		UID: 123, Nickname: "测试账号", CookiePath: filepath.Join(filepath.Dir(path), "fan1.json"),
		AccountPath: "${HOME}/fan1.json", Main: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(updated.Raw, "${HOME}/fan1.json") || !strings.Contains(updated.Raw, "昵称: 测试账号") || !strings.Contains(updated.Raw, "# keep this comment") {
		t.Fatalf("account update did not preserve expected YAML:\n%s", updated.Raw)
	}
	if _, err := store.updateAccount(document.Revision, loginresult.Result{
		UID: 456, AccountPath: "${HOME}/fan2.json", CookiePath: "fan2.json",
	}); !errors.Is(err, errConfigRevisionConflict) {
		t.Fatalf("stale account update error = %v; want revision conflict", err)
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
		jobRuns: make(map[string]map[string]struct{}),
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
