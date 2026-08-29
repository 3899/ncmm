package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const sessionTouchInterval = time.Minute

type Manager struct {
	store *store
	now   func() time.Time
}

func NewManager(path string) (*Manager, error) {
	m := &Manager{store: newStore(path), now: time.Now}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := m.store.view(ctx, func(storeData) error { return nil }); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) Configured(ctx context.Context) (bool, error) {
	configured := false
	err := m.store.view(ctx, func(data storeData) error {
		configured = data.PasswordHash != ""
		return nil
	})
	return configured, err
}

func (m *Manager) Status(ctx context.Context, token string) (Status, error) {
	var status Status
	err := m.store.view(ctx, func(data storeData) error {
		status.Configured = data.PasswordHash != ""
		status.Settings = data.Settings
		return nil
	})
	if err != nil || token == "" {
		return status, err
	}
	session, err := m.Authenticate(ctx, token)
	if err != nil {
		if err == ErrInvalidSession {
			return status, nil
		}
		return status, err
	}
	status.Authenticated = true
	status.CSRFToken = session.CSRFToken
	status.Session = SessionView{ID: session.ID, Current: true, ExpiresAt: session.ExpiresAt}
	return status, nil
}

func (m *Manager) Setup(ctx context.Context, password string, client ClientInfo) (Credentials, error) {
	settings := DefaultSettings()
	hash, err := HashPassword(password, settings)
	if err != nil {
		return Credentials{}, err
	}
	credentials, err := newCredentials(m.now().UTC(), settings, client)
	if err != nil {
		return Credentials{}, err
	}
	err = m.store.update(ctx, func(data *storeData) (bool, error) {
		if data.PasswordHash != "" {
			return false, ErrAlreadyConfigured
		}
		data.PasswordHash = hash
		data.Settings = settings
		data.Sessions = map[string]Session{credentials.Session.ID: credentials.Session}
		return true, nil
	})
	return credentials, err
}

func (m *Manager) Login(ctx context.Context, password string, client ClientInfo) (Credentials, error) {
	var credentials Credentials
	err := m.store.update(ctx, func(data *storeData) (bool, error) {
		if data.PasswordHash == "" {
			return false, ErrNotConfigured
		}
		valid, err := VerifyPassword(password, data.PasswordHash)
		if err != nil {
			return false, err
		}
		if !valid {
			return false, ErrInvalidCredentials
		}
		credentials, err = newCredentials(m.now().UTC(), data.Settings, client)
		if err != nil {
			return false, err
		}
		cleanupSessions(data, m.now().UTC())
		evictOldestSessions(data, maxSessions-1)
		data.Sessions[credentials.Session.ID] = credentials.Session
		return true, nil
	})
	return credentials, err
}

func (m *Manager) Authenticate(ctx context.Context, token string) (AuthenticatedSession, error) {
	if len(token) != 43 {
		return AuthenticatedSession{}, ErrInvalidSession
	}
	tokenHash := HashSessionToken(token)
	var authenticated AuthenticatedSession
	err := m.store.update(ctx, func(data *storeData) (bool, error) {
		now := m.now().UTC()
		dirty := cleanupSessions(data, now)
		for id, session := range data.Sessions {
			if !constantStringEqual(session.TokenHash, tokenHash) {
				continue
			}
			idleTimeout := timeSeconds(data.Settings.IdleTimeoutSeconds)
			if !now.Before(session.ExpiresAt) || (idleTimeout > 0 && now.Sub(session.LastSeen) > idleTimeout) {
				delete(data.Sessions, id)
				return true, nil
			}
			if now.Sub(session.LastSeen) >= sessionTouchInterval {
				session.LastSeen = now
				data.Sessions[id] = session
				dirty = true
			}
			authenticated = AuthenticatedSession{ID: id, CSRFToken: session.CSRFToken, ExpiresAt: session.ExpiresAt}
			return dirty, nil
		}
		return dirty, nil
	})
	if err == nil && authenticated.ID == "" {
		err = ErrInvalidSession
	}
	return authenticated, err
}

func (m *Manager) ChangePassword(ctx context.Context, newPassword string, client ClientInfo) (Credentials, error) {
	var settings Settings
	if err := m.store.view(ctx, func(data storeData) error {
		if data.PasswordHash == "" {
			return ErrNotConfigured
		}
		settings = data.Settings
		return nil
	}); err != nil {
		return Credentials{}, err
	}
	newHash, err := HashPassword(newPassword, settings)
	if err != nil {
		return Credentials{}, err
	}
	credentials, err := newCredentials(m.now().UTC(), settings, client)
	if err != nil {
		return Credentials{}, err
	}
	err = m.store.update(ctx, func(data *storeData) (bool, error) {
		if err := ValidatePassword(newPassword, data.Settings); err != nil {
			return false, err
		}
		data.PasswordHash = newHash
		data.Sessions = map[string]Session{credentials.Session.ID: credentials.Session}
		return true, nil
	})
	return credentials, err
}

func (m *Manager) ResetPassword(ctx context.Context, password string) error {
	var settings Settings
	if err := m.store.view(ctx, func(data storeData) error {
		settings = data.Settings
		return nil
	}); err != nil {
		return err
	}
	hash, err := HashPassword(password, settings)
	if err != nil {
		return err
	}
	return m.store.update(ctx, func(data *storeData) (bool, error) {
		if err := ValidatePassword(password, data.Settings); err != nil {
			return false, err
		}
		data.PasswordHash = hash
		data.Sessions = make(map[string]Session)
		return true, nil
	})
}

func (m *Manager) UpdateSettings(ctx context.Context, settings Settings) error {
	if err := ValidateSettings(settings); err != nil {
		return err
	}
	return m.store.update(ctx, func(data *storeData) (bool, error) {
		if data.PasswordHash == "" {
			return false, ErrNotConfigured
		}
		data.Settings = settings
		return true, nil
	})
}

func (m *Manager) ListSessions(ctx context.Context, currentID string) ([]SessionView, error) {
	var result []SessionView
	err := m.store.update(ctx, func(data *storeData) (bool, error) {
		dirty := cleanupSessions(data, m.now().UTC())
		result = make([]SessionView, 0, len(data.Sessions))
		for _, session := range data.Sessions {
			result = append(result, sessionView(session, session.ID == currentID))
		}
		sort.Slice(result, func(i, j int) bool { return result[i].LastSeen.After(result[j].LastSeen) })
		return dirty, nil
	})
	return result, err
}

func (m *Manager) RevokeSession(ctx context.Context, id string) error {
	return m.store.update(ctx, func(data *storeData) (bool, error) {
		if _, ok := data.Sessions[id]; !ok {
			return false, ErrSessionNotFound
		}
		delete(data.Sessions, id)
		return true, nil
	})
}

func (m *Manager) RevokeOtherSessions(ctx context.Context, currentID string) error {
	return m.store.update(ctx, func(data *storeData) (bool, error) {
		current, ok := data.Sessions[currentID]
		if !ok {
			return false, ErrInvalidSession
		}
		data.Sessions = map[string]Session{currentID: current}
		return true, nil
	})
}

func (m *Manager) RevokeToken(ctx context.Context, token string) error {
	if len(token) != 43 {
		return nil
	}
	hash := HashSessionToken(token)
	return m.store.update(ctx, func(data *storeData) (bool, error) {
		for id, session := range data.Sessions {
			if constantStringEqual(session.TokenHash, hash) {
				delete(data.Sessions, id)
				return true, nil
			}
		}
		return false, nil
	})
}

func (m *Manager) Clear(ctx context.Context) error {
	return m.store.clear(ctx)
}

func RecoverPassword(ctx context.Context, path, password string) error {
	settings := DefaultSettings()
	hash, err := HashPassword(password, settings)
	if err != nil {
		return err
	}
	return newStore(path).replace(ctx, storeData{
		Version: StoreVersion, PasswordHash: hash,
		Settings: settings, Sessions: make(map[string]Session),
	})
}

func ClearStore(ctx context.Context, path string) error {
	return newStore(path).clear(ctx)
}

func newCredentials(now time.Time, settings Settings, client ClientInfo) (Credentials, error) {
	token, err := randomToken(32)
	if err != nil {
		return Credentials{}, err
	}
	csrf, err := randomToken(24)
	if err != nil {
		return Credentials{}, err
	}
	expires := now.Add(timeSeconds(settings.SessionTTLSeconds))
	session := Session{
		ID: uuid.NewString(), TokenHash: HashSessionToken(token), CSRFToken: csrf,
		CreatedAt: now, ExpiresAt: expires, LastSeen: now,
		IP: truncate(client.IP, 64), UserAgent: truncate(client.UserAgent, 256),
	}
	return Credentials{Token: token, Session: session, Settings: settings, ExpiresAt: expires}, nil
}

func randomToken(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func HashSessionToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func cleanupSessions(data *storeData, now time.Time) bool {
	dirty := false
	idle := timeSeconds(data.Settings.IdleTimeoutSeconds)
	for id, session := range data.Sessions {
		if !now.Before(session.ExpiresAt) || (idle > 0 && now.Sub(session.LastSeen) > idle) {
			delete(data.Sessions, id)
			dirty = true
		}
	}
	return dirty
}

func evictOldestSessions(data *storeData, keep int) {
	for len(data.Sessions) > keep {
		oldestID := ""
		var oldest time.Time
		for id, session := range data.Sessions {
			if oldestID == "" || session.LastSeen.Before(oldest) {
				oldestID = id
				oldest = session.LastSeen
			}
		}
		delete(data.Sessions, oldestID)
	}
}

func sessionView(session Session, current bool) SessionView {
	return SessionView{
		ID: session.ID, Current: current, CreatedAt: session.CreatedAt,
		ExpiresAt: session.ExpiresAt, LastSeen: session.LastSeen,
		IP: session.IP, UserAgent: session.UserAgent,
	}
}

func constantStringEqual(left, right string) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func timeSeconds(seconds int64) time.Duration {
	return time.Duration(seconds) * time.Second
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	for len(value) > max {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value
}
