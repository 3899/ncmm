package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testPassword    = "Admin#123"
	changedPassword = "Changed#456"
)

func TestPasswordHashUsesSimAdminASCIIRules(t *testing.T) {
	settings := DefaultSettings()
	hash, err := HashPassword(testPassword, settings)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hash, testPassword) {
		t.Fatal("password hash contains plaintext")
	}
	valid, err := VerifyPassword(testPassword, hash)
	if err != nil || !valid {
		t.Fatalf("VerifyPassword(valid) = %v, %v", valid, err)
	}
	valid, err = VerifyPassword("Wrong#123", hash)
	if err != nil || valid {
		t.Fatalf("VerifyPassword(invalid) = %v, %v", valid, err)
	}
	for _, password := range []string{
		"short", "onlyletters", "letters123", "12345678#",
		"Admin 123#", "Admin中文123#", strings.Repeat("A", maxPasswordLength+1) + "1#",
	} {
		if err := ValidatePassword(password, settings); err == nil {
			t.Fatalf("ValidatePassword(%q) expected error", password)
		}
	}
}

func TestSettingsRejectDurationOverflow(t *testing.T) {
	settings := DefaultSettings()
	settings.SessionTTLSeconds = math.MaxInt64
	if err := ValidateSettings(settings); err == nil {
		t.Fatal("overflowing session TTL was accepted")
	}
	settings = DefaultSettings()
	settings.IdleTimeoutSeconds = math.MaxInt64
	if err := ValidateSettings(settings); err == nil {
		t.Fatal("overflowing idle timeout was accepted")
	}
}

func TestManagerSetupLoginAndStoreContainNoPlaintextSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultStoreName)
	manager := newTestManager(t, path)
	setup, err := manager.Setup(context.Background(), testPassword, ClientInfo{IP: "127.0.0.1", UserAgent: "test-agent"})
	if err != nil {
		t.Fatal(err)
	}
	if setup.Token == "" || setup.Session.CSRFToken == "" {
		t.Fatalf("incomplete setup credentials: %+v", setup)
	}
	if _, err := manager.Authenticate(context.Background(), setup.Token); err != nil {
		t.Fatal(err)
	}
	login, err := manager.Login(context.Background(), testPassword, ClientInfo{IP: "192.0.2.10"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Authenticate(context.Background(), login.Token); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{testPassword, setup.Token, login.Token} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("authentication store contains plaintext secret %q", secret)
		}
	}
	var stored storeData
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatal(err)
	}
	if len(stored.Sessions) != 2 || !strings.HasPrefix(stored.PasswordHash, passwordAlgorithm+"$") {
		t.Fatalf("unexpected store data: %+v", stored)
	}
}

func TestManagerConcurrentSetupHasOneWinner(t *testing.T) {
	manager := newTestManager(t, filepath.Join(t.TempDir(), DefaultStoreName))
	const attempts = 4
	start := make(chan struct{})
	results := make(chan error, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := manager.Setup(context.Background(), testPassword, ClientInfo{})
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	var succeeded, conflicted int
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrAlreadyConfigured):
			conflicted++
		default:
			t.Fatalf("unexpected setup error: %v", err)
		}
	}
	if succeeded != 1 || conflicted != attempts-1 {
		t.Fatalf("setup results: succeeded=%d conflicted=%d", succeeded, conflicted)
	}
}

func TestChangePasswordRevokesEveryOldSession(t *testing.T) {
	manager := newTestManager(t, filepath.Join(t.TempDir(), DefaultStoreName))
	first, err := manager.Setup(context.Background(), testPassword, ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Login(context.Background(), testPassword, ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}
	changed, err := manager.ChangePassword(context.Background(), changedPassword, ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}
	for _, old := range []string{first.Token, second.Token} {
		if _, err := manager.Authenticate(context.Background(), old); !errors.Is(err, ErrInvalidSession) {
			t.Fatalf("old session authentication error = %v", err)
		}
	}
	if _, err := manager.Authenticate(context.Background(), changed.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Login(context.Background(), testPassword, ClientInfo{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password login error = %v", err)
	}
	if _, err := manager.Login(context.Background(), changedPassword, ClientInfo{}); err != nil {
		t.Fatal(err)
	}
}

func TestSessionExpiryIdleTimeoutAndTouch(t *testing.T) {
	manager := newTestManager(t, filepath.Join(t.TempDir(), DefaultStoreName))
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	credentials, err := manager.Setup(context.Background(), testPassword, ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(2 * time.Minute)
	if _, err := manager.Authenticate(context.Background(), credentials.Token); err != nil {
		t.Fatal(err)
	}
	sessions, err := manager.ListSessions(context.Background(), credentials.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || !sessions[0].LastSeen.Equal(now) {
		t.Fatalf("session touch was not persisted: %+v", sessions)
	}

	now = now.Add(time.Hour + time.Second)
	if _, err := manager.Authenticate(context.Background(), credentials.Token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("idle session error = %v", err)
	}

	manager.now = func() time.Time { return credentials.ExpiresAt.Add(time.Second) }
	if _, err := manager.Authenticate(context.Background(), credentials.Token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expired session error = %v", err)
	}
}

func TestSessionListingAndRevocation(t *testing.T) {
	manager := newTestManager(t, filepath.Join(t.TempDir(), DefaultStoreName))
	first, err := manager.Setup(context.Background(), testPassword, ClientInfo{IP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Login(context.Background(), testPassword, ClientInfo{IP: "192.0.2.2"})
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := manager.ListSessions(context.Background(), first.Session.ID)
	if err != nil || len(sessions) != 2 {
		t.Fatalf("ListSessions() = %+v, %v", sessions, err)
	}
	if err := manager.RevokeSession(context.Background(), second.Session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Authenticate(context.Background(), second.Token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("revoked session error = %v", err)
	}
	third, err := manager.Login(context.Background(), testPassword, ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RevokeOtherSessions(context.Background(), first.Session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Authenticate(context.Background(), third.Token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("other session error = %v", err)
	}
	if _, err := manager.Authenticate(context.Background(), first.Token); err != nil {
		t.Fatal(err)
	}
}

func TestManagerFailsClosedForCorruptStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultStoreName)
	if err := os.WriteFile(path, []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewManager(path); err == nil || !strings.Contains(err.Error(), "parse authentication store") {
		t.Fatalf("NewManager() error = %v", err)
	}
}

func TestOfflineRecoveryReplacesCorruptStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultStoreName)
	if err := os.WriteFile(path, []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := RecoverPassword(context.Background(), path, changedPassword); err != nil {
		t.Fatal(err)
	}
	manager := newTestManager(t, path)
	if _, err := manager.Login(context.Background(), changedPassword, ClientInfo{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("corrupt-again"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ClearStore(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("store remains after clear: %v", err)
	}
}

func TestLoginCapsStoredSessions(t *testing.T) {
	manager := newTestManager(t, filepath.Join(t.TempDir(), DefaultStoreName))
	if _, err := manager.Setup(context.Background(), testPassword, ClientInfo{}); err != nil {
		t.Fatal(err)
	}
	if err := manager.store.update(context.Background(), func(data *storeData) (bool, error) {
		now := time.Now().UTC()
		for i := 0; i < maxSessions+5; i++ {
			id := fmt.Sprintf("historical-%03d", i)
			data.Sessions[id] = Session{
				ID: id, TokenHash: HashSessionToken(id), CSRFToken: id,
				CreatedAt: now.Add(time.Duration(i) * time.Second),
				LastSeen:  now.Add(time.Duration(i) * time.Second),
				ExpiresAt: now.Add(24 * time.Hour),
			}
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Login(context.Background(), testPassword, ClientInfo{}); err != nil {
		t.Fatal(err)
	}
	sessions, err := manager.ListSessions(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != maxSessions {
		t.Fatalf("stored sessions = %d; want %d", len(sessions), maxSessions)
	}
}

func newTestManager(t *testing.T, path string) *Manager {
	t.Helper()
	manager, err := NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	return manager
}
