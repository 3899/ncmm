package webui

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestQRCodeLoginHelperProcess(t *testing.T) {
	if len(os.Args) < 2 || os.Args[len(os.Args)-2] != "--qrcode-login-helper" {
		return
	}
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(filepath.Join(os.Args[len(os.Args)-1], "qrcode.png"), data, 0600); err != nil {
		os.Exit(2)
	}
	time.Sleep(250 * time.Millisecond)
	os.Exit(0)
}

func TestQRCodeLoginManagerPassesOutputAndCachesImage(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := newQRCodeLoginManager(ctx, os.Args[0], filepath.Join(home, "config.yaml"), home)
	defer manager.close()

	var gotArgs []string
	manager.command = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		dir := args[slices.Index(args, "--dir")+1]
		return exec.CommandContext(ctx, os.Args[0], "-test.run=^TestQRCodeLoginHelperProcess$", "--", "--qrcode-login-helper", dir)
	}

	view, err := manager.start("custom.json", true)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !qrcodeLoginTerminal(view.Status) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		view, _ = manager.get(view.ID)
	}
	if view.Status != "succeeded" || !view.ImageReady {
		t.Fatalf("unexpected qrcode session: %+v", view)
	}
	if outputIndex := slices.Index(gotArgs, "--output"); outputIndex < 0 || outputIndex+1 >= len(gotArgs) || gotArgs[outputIndex+1] != "custom.json" {
		t.Fatalf("qrcode args do not include expected output: %q", gotArgs)
	}
	if !slices.Contains(gotArgs, "--main") {
		t.Fatalf("qrcode args do not include --main: %q", gotArgs)
	}
	if image, ok := manager.image(view.ID); !ok || !isPNG(image) {
		t.Fatal("qrcode image was not cached")
	}
}

func TestQRCodeLoginManagerRejectsConcurrentSession(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := newQRCodeLoginManager(ctx, os.Args[0], filepath.Join(home, "config.yaml"), home)
	defer manager.close()
	manager.command = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		dir := args[slices.Index(args, "--dir")+1]
		return exec.CommandContext(ctx, os.Args[0], "-test.run=^TestQRCodeLoginHelperProcess$", "--", "--qrcode-login-helper", dir)
	}
	if _, err := manager.start("first.json", false); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.start("second.json", false); err != errQRCodeLoginActive {
		t.Fatalf("concurrent start error = %v; want %v", err, errQRCodeLoginActive)
	}
}
