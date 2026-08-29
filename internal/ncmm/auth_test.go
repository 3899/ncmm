package ncmm

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/3899/ncmm/internal/filelock"
	"github.com/3899/ncmm/internal/webui"
	webauth "github.com/3899/ncmm/internal/webui/auth"
)

func TestAuthResetPasswordAndClearCommands(t *testing.T) {
	home := t.TempDir()
	manager, err := webauth.NewManager(filepath.Join(home, webauth.DefaultStoreName))
	if err != nil {
		t.Fatal(err)
	}
	old, err := manager.Setup(context.Background(), "Original#123", webauth.ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}

	output := new(bytes.Buffer)
	root := New()
	root.cmd.SetOut(output)
	root.cmd.SetErr(output)
	root.cmd.SetIn(strings.NewReader("Changed#456\n"))
	root.cmd.SetArgs([]string{"--home", home, "auth", "reset-password", "--password-stdin"})
	if err := root.cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "all browser sessions were revoked") {
		t.Fatalf("reset output = %q", output.String())
	}
	if _, err := manager.Authenticate(context.Background(), old.Token); !errors.Is(err, webauth.ErrInvalidSession) {
		t.Fatalf("old session error = %v", err)
	}
	if _, err := manager.Login(context.Background(), "Changed#456", webauth.ClientInfo{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "webui.secret"), []byte("legacy-token-123\n"), 0600); err != nil {
		t.Fatal(err)
	}
	taskData := map[string]string{
		"config.yaml":            "version: 1.2.0\n",
		"cookie.json":            `[{"name":"MUSIC_U","value":"preserve-me"}]`,
		"database/000001.sst":    "database-content",
		"webui.yaml":             "version: 1\njobs: []\n",
		"log/runs/run/meta.json": `{"status":"success"}`,
	}
	for name, content := range taskData {
		path := filepath.Join(home, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	root = New()
	root.cmd.SetOut(output)
	root.cmd.SetErr(output)
	root.cmd.SetArgs([]string{"--home", home, "auth", "clear", "--yes"})
	if err := root.cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	configured, err := manager.Configured(context.Background())
	if err != nil || configured {
		t.Fatalf("configured after clear = %v, %v", configured, err)
	}
	legacy, err := os.ReadFile(filepath.Join(home, "webui.secret"))
	if err != nil || string(legacy) != "legacy-token-123\n" {
		t.Fatalf("unrelated legacy file changed after clear: %q, %v", legacy, err)
	}
	for name, want := range taskData {
		got, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(name)))
		if err != nil || string(got) != want {
			t.Fatalf("task data %q changed after auth clear: %q, %v", name, got, err)
		}
	}
}

func TestAuthRecoveryRequiresStoppedWebUI(t *testing.T) {
	home := t.TempDir()
	lock, err := filelock.TryAcquire(filepath.Join(home, webui.InstanceLockFilename))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()

	root := New()
	root.cmd.SetIn(strings.NewReader("Changed#456\n"))
	root.cmd.SetArgs([]string{"--home", home, "auth", "reset-password", "--password-stdin"})
	err = root.cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "WebUI is running") {
		t.Fatalf("reset while running error = %v", err)
	}
}

func TestAuthClearRequiresConfirmation(t *testing.T) {
	root := New()
	root.cmd.SetArgs([]string{"--home", t.TempDir(), "auth", "clear"})
	if err := root.cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("clear error = %v", err)
	}
}

func TestReadRecoveryPasswordRejectsMultipleSources(t *testing.T) {
	t.Setenv("NCMM_WEB_ADMIN_PASSWORD", "Environment#123")
	file := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(file, []byte("File#123\n"), 0600); err != nil {
		t.Fatal(err)
	}
	command := newAuthResetPasswordCommand(New())
	if _, err := readRecoveryPassword(command, authPasswordOptions{file: file}); err == nil {
		t.Fatal("multiple password sources were accepted")
	}
}
