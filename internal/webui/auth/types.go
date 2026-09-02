package auth

import (
	"errors"
	"time"
)

const (
	StoreVersion      = 1
	SessionCookieName = "ncmm_session"
	DefaultStoreName  = "webui-auth.json"
	maxPasswordLength = 64
	minSessionTTL     = 15 * time.Minute
	maxSessionTTL     = 90 * 24 * time.Hour
	maxIdleTimeout    = 30 * 24 * time.Hour
	maxSessions       = 128
)

var (
	ErrAlreadyConfigured  = errors.New("administrator password is already configured")
	ErrNotConfigured      = errors.New("administrator password is not configured")
	ErrInvalidCredentials = errors.New("invalid administrator password")
	ErrInvalidSession     = errors.New("invalid or expired session")
	ErrSessionNotFound    = errors.New("session not found")
)

type Settings struct {
	PasswordMinLength      int   `json:"passwordMinLength"`
	PasswordRequireLetters bool  `json:"passwordRequireLetters"`
	PasswordRequireDigits  bool  `json:"passwordRequireDigits"`
	PasswordRequireSymbols bool  `json:"passwordRequireSymbols"`
	SessionTTLSeconds      int64 `json:"sessionTTLSeconds"`
	IdleTimeoutSeconds     int64 `json:"idleTimeoutSeconds"`
}

func DefaultSettings() Settings {
	return Settings{
		PasswordMinLength:      1,
		PasswordRequireLetters: false,
		PasswordRequireDigits:  false,
		PasswordRequireSymbols: false,
		SessionTTLSeconds:      int64((7 * 24 * time.Hour) / time.Second),
		IdleTimeoutSeconds:     int64(time.Hour / time.Second),
	}
}

type ClientInfo struct {
	IP        string
	UserAgent string
}

type Session struct {
	ID        string    `json:"id"`
	TokenHash string    `json:"tokenHash"`
	CSRFToken string    `json:"csrfToken"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	LastSeen  time.Time `json:"lastSeen"`
	IP        string    `json:"ip,omitempty"`
	UserAgent string    `json:"userAgent,omitempty"`
}

type SessionView struct {
	ID        string    `json:"id"`
	Current   bool      `json:"current"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	LastSeen  time.Time `json:"lastSeen"`
	IP        string    `json:"ip,omitempty"`
	UserAgent string    `json:"userAgent,omitempty"`
}

type Credentials struct {
	Token     string
	Session   Session
	Settings  Settings
	ExpiresAt time.Time
}

type AuthenticatedSession struct {
	ID        string
	CSRFToken string
	ExpiresAt time.Time
}

type Status struct {
	Configured                bool        `json:"configured"`
	Authenticated             bool        `json:"authenticated"`
	PasswordProtectionEnabled bool        `json:"passwordProtectionEnabled"`
	CSRFToken                 string      `json:"csrfToken,omitempty"`
	Settings                  Settings    `json:"settings"`
	Session                   SessionView `json:"session,omitempty"`
}
