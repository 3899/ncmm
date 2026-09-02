package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	webauth "github.com/3899/ncmm/internal/webui/auth"
)

func TestPasswordLoginUsesCookieAndCSRF(t *testing.T) {
	server := newAuthTestServer(t, t.TempDir())
	credentials, err := server.authManager.Setup(context.Background(), "Admin#123", requestClientInfo(httptest.NewRequest(http.MethodGet, "/", nil)))
	if err != nil {
		t.Fatal(err)
	}
	handler := server.routes()

	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"Admin#123"}`))
	loginRequest.Host = "localhost:3899"
	loginRequest.Header.Set("Content-Type", "application/json")
	loginRequest.Header.Set("Origin", "http://localhost:3899")
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginResponse.Code, loginResponse.Body.String())
	}
	var loginPayload struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &loginPayload); err != nil {
		t.Fatal(err)
	}
	cookies := loginResponse.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value == credentials.Token || loginPayload.CSRFToken == "" {
		t.Fatalf("unexpected login credentials: cookies=%+v payload=%+v", cookies, loginPayload)
	}

	settingsBody := `{"passwordMinLength":8,"passwordRequireLetters":true,"passwordRequireDigits":true,"passwordRequireSymbols":true,"sessionTTLSeconds":604800,"idleTimeoutSeconds":3600}`
	request := func(csrf, origin string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/settings", strings.NewReader(settingsBody))
		req.Host = "localhost:3899"
		req.Header.Set("Content-Type", "application/json")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if csrf != "" {
			req.Header.Set(csrfHeader, csrf)
		}
		req.AddCookie(cookies[0])
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response.Code
	}
	if got := request("", "http://localhost:3899"); got != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d", got)
	}
	if got := request(loginPayload.CSRFToken, "http://attacker.example"); got != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d", got)
	}
	if got := request(loginPayload.CSRFToken, "http://localhost:3899"); got != http.StatusOK {
		t.Fatalf("valid settings update status = %d", got)
	}
}

func TestAuthStatusContainsOnlyNewSetupContract(t *testing.T) {
	server := newAuthTestServer(t, t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	req.Host = "localhost:3899"
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("auth status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["configured"] != false || payload["setupRequired"] != true {
		t.Fatalf("unexpected setup status: %#v", payload)
	}
	for _, removed := range []string{"bootstrapRequired", "bootstrapCode", "legacyCredentialEnabled"} {
		if _, exists := payload[removed]; exists {
			t.Fatalf("removed auth status field %q is still present: %#v", removed, payload)
		}
	}
}

func TestPasswordProtectionToggleChangesMiddlewareBehavior(t *testing.T) {
	server := newAuthTestServer(t, t.TempDir())
	credentials, err := server.authManager.Setup(context.Background(), "a", requestClientInfo(httptest.NewRequest(http.MethodGet, "/", nil)))
	if err != nil {
		t.Fatal(err)
	}
	handler := server.routes()
	settings := `{"passwordProtectionEnabled":false,"passwordMinLength":1,"passwordRequireLetters":false,"passwordRequireDigits":false,"passwordRequireSymbols":false,"sessionTTLSeconds":604800,"idleTimeoutSeconds":3600}`
	request := httptest.NewRequest(http.MethodPut, "/api/v1/auth/settings", strings.NewReader(settings))
	request.Host = "localhost:3899"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(csrfHeader, credentials.Session.CSRFToken)
	request.AddCookie(&http.Cookie{Name: webauth.SessionCookieName, Value: credentials.Token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/settings", nil)
	request.Host = "localhost:3899"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unprotected API status=%d body=%s", response.Code, response.Body.String())
	}
	settings = strings.Replace(settings, `"passwordProtectionEnabled":false`, `"passwordProtectionEnabled":true`, 1)
	request = httptest.NewRequest(http.MethodPut, "/api/v1/auth/settings", strings.NewReader(settings))
	request.Host = "localhost:3899"
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("re-enable status=%d body=%s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/settings", nil)
	request.Host = "localhost:3899"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("protected API without session status=%d", response.Code)
	}
}

func TestUnconfiguredRemoteListenerAllowsFirstSetup(t *testing.T) {
	home := t.TempDir()
	server, err := New(context.Background(), Options{Home: home, Listen: "0.0.0.0:3899"})
	if err != nil {
		t.Fatalf("unconfigured remote New() error = %v", err)
	}
	defer server.scheduler.close()
	defer server.qrcode.close()
	configured, err := server.authManager.Configured(context.Background())
	if err != nil || configured {
		t.Fatalf("initial configured state = %v, %v", configured, err)
	}
}

func TestLegacySecretIsIgnoredAndBearerIsRejected(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "webui.secret"), []byte("legacy-token-123\n"), 0600); err != nil {
		t.Fatal(err)
	}
	server, err := New(context.Background(), Options{Home: home, Listen: "0.0.0.0:3899"})
	if err != nil {
		t.Fatal(err)
	}
	defer server.scheduler.close()
	defer server.qrcode.close()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Host = "localhost:3899"
	req.Header.Set("Authorization", "Bearer legacy-token-123")
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, req)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("legacy Bearer status = %d; want %d", response.Code, http.StatusUnauthorized)
	}
	configured, err := server.authManager.Configured(context.Background())
	if err != nil || configured {
		t.Fatalf("legacy secret affected setup state: configured=%v, err=%v", configured, err)
	}
}

func TestFrontendDoesNotPersistAuthenticationCredentials(t *testing.T) {
	script, err := staticFiles.ReadFile("static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"sessionStorage", "ncmm-token", "Authorization: `Bearer", "bootstrap", "legacyToken"} {
		if strings.Contains(string(script), forbidden) {
			t.Fatalf("frontend still contains credential persistence pattern %q", forbidden)
		}
	}
}

func TestSecureCookieOptionAndLogout(t *testing.T) {
	server := newAuthTestServer(t, t.TempDir())
	server.opts.SecureCookie = true
	credentials, err := server.authManager.Setup(context.Background(), "Admin#123", webauth.ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server.setSessionCookie(recorder, credentials)
	cookie := recorder.Result().Cookies()[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" {
		t.Fatalf("unexpected secure cookie: %+v", cookie)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Host = "localhost:3899"
	req.Header.Set("Origin", "http://localhost:3899")
	req.Header.Set(csrfHeader, credentials.Session.CSRFToken)
	req.AddCookie(cookie)
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("logout status = %d, body = %s", response.Code, response.Body.String())
	}
	expired := response.Result().Cookies()[0]
	if expired.MaxAge >= 0 || !expired.Secure {
		t.Fatalf("unexpected expired cookie: %+v", expired)
	}
	if _, err := server.authManager.Authenticate(context.Background(), credentials.Token); err == nil {
		t.Fatal("logged-out session is still valid")
	}
}

func TestLoginRateLimiterBlocksAndRefills(t *testing.T) {
	limiter := newLoginRateLimiter()
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }
	limiter.global = loginBucket{tokens: 20, last: now}
	for i := 0; i < 5; i++ {
		if allowed, _ := limiter.allow("192.0.2.1"); !allowed {
			t.Fatalf("attempt %d was unexpectedly blocked", i+1)
		}
	}
	if allowed, retry := limiter.allow("192.0.2.1"); allowed || retry < 59*time.Second {
		t.Fatalf("sixth attempt = allowed %v, retry %v", allowed, retry)
	}
	if allowed, _ := limiter.allow("192.0.2.2"); !allowed {
		t.Fatal("one blocked client affected another before the global limit")
	}
	now = now.Add(time.Minute)
	if allowed, _ := limiter.allow("192.0.2.1"); !allowed {
		t.Fatal("client bucket did not refill")
	}
}

func newAuthTestServer(t *testing.T, home string) *Server {
	t.Helper()
	manager, err := webauth.NewManager(filepath.Join(home, webauth.DefaultStoreName))
	if err != nil {
		t.Fatal(err)
	}
	return &Server{
		opts:        Options{Home: home, Listen: defaultListen},
		authManager: manager, loginLimiter: newLoginRateLimiter(),
	}
}
