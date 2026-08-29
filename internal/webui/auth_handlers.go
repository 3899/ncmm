package webui

import (
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	webauth "github.com/3899/ncmm/internal/webui/auth"
)

const csrfHeader = "X-NCMM-CSRF"

type authStatusResponse struct {
	webauth.Status
	SetupRequired bool   `json:"setupRequired"`
	SecureCookie  bool   `json:"secureCookie"`
	Version       string `json:"version"`
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.authManager.Status(r.Context(), sessionToken(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	response := authStatusResponse{
		Status: status, SetupRequired: !status.Configured,
		SecureCookie: s.opts.SecureCookie, Version: s.opts.Version,
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	if !s.consumeLoginAttempt(w, r) {
		return
	}
	configured, err := s.authManager.Configured(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if configured {
		writeError(w, http.StatusConflict, webauth.ErrAlreadyConfigured.Error())
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req, 8<<10) {
		return
	}
	credentials, err := s.authManager.Setup(r.Context(), req.Password, requestClientInfo(r))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, webauth.ErrAlreadyConfigured) {
			status = http.StatusConflict
		}
		writeError(w, status, err.Error())
		return
	}
	s.setSessionCookie(w, credentials)
	writeJSON(w, http.StatusOK, map[string]any{
		"configured": true, "authenticated": true,
		"csrfToken": credentials.Session.CSRFToken, "settings": credentials.Settings,
	})
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if !s.consumeLoginAttempt(w, r) {
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req, 4<<10) {
		return
	}
	credentials, err := s.authManager.Login(r.Context(), req.Password, requestClientInfo(r))
	if err != nil {
		if errors.Is(err, webauth.ErrNotConfigured) {
			writeError(w, http.StatusConflict, "管理员密码尚未设置")
			return
		}
		if errors.Is(err, webauth.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "管理员密码不正确")
			return
		}
		writeError(w, http.StatusInternalServerError, "verify administrator password: "+err.Error())
		return
	}
	s.setSessionCookie(w, credentials)
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true, "csrfToken": credentials.Session.CSRFToken,
		"settings": credentials.Settings,
	})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	token := sessionToken(r)
	if token != "" {
		session, err := s.authManager.Authenticate(r.Context(), token)
		if err == nil {
			provided := r.Header.Get(csrfHeader)
			if !constantStringEqual(provided, session.CSRFToken) {
				writeError(w, http.StatusForbidden, "invalid CSRF token")
				return
			}
			_ = s.authManager.RevokeToken(r.Context(), token)
		}
	}
	s.expireSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"loggedOut": true})
}

func (s *Server) handleAuthPasswordPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NewPassword string `json:"newPassword"`
	}
	if !decodeJSON(w, r, &req, 8<<10) {
		return
	}
	credentials, err := s.authManager.ChangePassword(r.Context(), req.NewPassword, requestClientInfo(r))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.setSessionCookie(w, credentials)
	writeJSON(w, http.StatusOK, map[string]any{
		"updated": true, "csrfToken": credentials.Session.CSRFToken,
		"settings": credentials.Settings,
	})
}

func (s *Server) handleAuthSettingsGet(w http.ResponseWriter, r *http.Request) {
	status, err := s.authManager.Status(r.Context(), sessionToken(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status.Settings)
}

func (s *Server) handleAuthSettingsPut(w http.ResponseWriter, r *http.Request) {
	var settings webauth.Settings
	if !decodeJSON(w, r, &settings, 8<<10) {
		return
	}
	if err := s.authManager.UpdateSettings(r.Context(), settings); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleAuthSessionsGet(w http.ResponseWriter, r *http.Request) {
	authenticated := requestAuthentication(r)
	sessions, err := s.authManager.ListSessions(r.Context(), authenticated.Session.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) handleAuthSessionDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.authManager.RevokeSession(r.Context(), id); err != nil {
		if errors.Is(err, webauth.ErrSessionNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if requestAuthentication(r).Session.ID == id {
		s.expireSessionCookie(w)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"revoked": true})
}

func (s *Server) handleAuthSessionsRevokeOthers(w http.ResponseWriter, r *http.Request) {
	currentID := requestAuthentication(r).Session.ID
	if currentID == "" {
		writeError(w, http.StatusBadRequest, "a browser session is required")
		return
	}
	if err := s.authManager.RevokeOtherSessions(r.Context(), currentID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"revoked": true})
}

func (s *Server) consumeLoginAttempt(w http.ResponseWriter, r *http.Request) bool {
	allowed, retry := s.loginLimiter.allow(clientIP(r.RemoteAddr))
	if allowed {
		return true
	}
	seconds := max(1, int((retry+time.Second-1)/time.Second))
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeError(w, http.StatusTooManyRequests, "too many authentication attempts; try again later")
	return false
}

func (s *Server) setSessionCookie(w http.ResponseWriter, credentials webauth.Credentials) {
	http.SetCookie(w, &http.Cookie{
		Name: webauth.SessionCookieName, Value: credentials.Token, Path: "/",
		HttpOnly: true, Secure: s.opts.SecureCookie, SameSite: http.SameSiteStrictMode,
		Expires: credentials.ExpiresAt, MaxAge: int(credentials.Settings.SessionTTLSeconds),
	})
}

func (s *Server) expireSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: webauth.SessionCookieName, Path: "/", HttpOnly: true,
		Secure: s.opts.SecureCookie, SameSite: http.SameSiteStrictMode,
		Expires: time.Unix(1, 0), MaxAge: -1,
	})
}

func sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(webauth.SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func requestClientInfo(r *http.Request) webauth.ClientInfo {
	return webauth.ClientInfo{IP: clientIP(r.RemoteAddr), UserAgent: r.UserAgent()}
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return strings.TrimSpace(remoteAddr)
	}
	return host
}

func constantStringEqual(left, right string) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
