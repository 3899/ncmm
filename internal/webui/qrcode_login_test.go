package webui

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/3899/ncmm/internal/loginresult"
)

func TestQRCodeLoginHelperProcess(t *testing.T) {
	if len(os.Args) < 4 || os.Args[len(os.Args)-4] != "--qrcode-login-helper" {
		return
	}
	dir := os.Args[len(os.Args)-3]
	filename := os.Args[len(os.Args)-2]
	main, err := strconv.ParseBool(os.Args[len(os.Args)-1])
	if err != nil {
		os.Exit(2)
	}
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(filepath.Join(dir, "qrcode.png"), data, 0600); err != nil {
		os.Exit(2)
	}
	time.Sleep(250 * time.Millisecond)
	if err := loginresult.Write(os.Stdout, loginresult.Result{
		UID: 123, Nickname: "tester", CookiePath: filepath.Join(dir, filename), AccountPath: filename, Main: main,
	}); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestQRCodeLoginManagerPassesOutputAndCachesImage(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var committedRevision string
	var committedResult loginresult.Result
	manager := newQRCodeLoginManager(ctx, os.Args[0], filepath.Join(home, "config.yaml"), home, func(revision string, result loginresult.Result) error {
		committedRevision = revision
		committedResult = result
		return nil
	})
	defer manager.close()

	var gotArgs []string
	manager.command = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		dir := args[slices.Index(args, "--dir")+1]
		filename := args[slices.Index(args, "--output")+1]
		main := slices.Contains(args, "--main")
		return exec.CommandContext(ctx, os.Args[0], "-test.run=^TestQRCodeLoginHelperProcess$", "--", "--qrcode-login-helper", dir, filename, strconv.FormatBool(main))
	}

	view, err := manager.start("custom.json", true, "revision-1")
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
	if !slices.Contains(gotArgs, "--no-config-write") || !slices.Contains(gotArgs, "--json-result") {
		t.Fatalf("qrcode args do not enforce parent-owned config: %q", gotArgs)
	}
	if committedRevision != "revision-1" || committedResult.AccountPath != "custom.json" || !committedResult.Main {
		t.Fatalf("unexpected committed result: revision=%q result=%+v", committedRevision, committedResult)
	}
	if image, ok := manager.image(view.ID); !ok || !isPNG(image) {
		t.Fatal("qrcode image was not cached")
	}
}

func TestQRCodeLoginManagerRejectsConcurrentSession(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := newQRCodeLoginManager(ctx, os.Args[0], filepath.Join(home, "config.yaml"), home, func(string, loginresult.Result) error { return nil })
	defer manager.close()
	manager.command = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		dir := args[slices.Index(args, "--dir")+1]
		filename := args[slices.Index(args, "--output")+1]
		return exec.CommandContext(ctx, os.Args[0], "-test.run=^TestQRCodeLoginHelperProcess$", "--", "--qrcode-login-helper", dir, filename, "false")
	}
	if _, err := manager.start("first.json", false, "revision-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.start("second.json", false, "revision-1"); err != errQRCodeLoginActive {
		t.Fatalf("concurrent start error = %v; want %v", err, errQRCodeLoginActive)
	}
}

func TestQRCodeLoginManagerReportsConfigConflict(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := newQRCodeLoginManager(ctx, os.Args[0], filepath.Join(home, "config.yaml"), home, func(string, loginresult.Result) error {
		return errConfigRevisionConflict
	})
	defer manager.close()
	manager.command = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		dir := args[slices.Index(args, "--dir")+1]
		filename := args[slices.Index(args, "--output")+1]
		return exec.CommandContext(ctx, os.Args[0], "-test.run=^TestQRCodeLoginHelperProcess$", "--", "--qrcode-login-helper", dir, filename, "false")
	}
	view, err := manager.start("fan1.json", false, "stale-revision")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !qrcodeLoginTerminal(view.Status) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		view, _ = manager.get(view.ID)
	}
	if view.Status != "failed" || !strings.Contains(view.Message, "配置提交失败") {
		t.Fatalf("unexpected conflict session: %+v", view)
	}
}
