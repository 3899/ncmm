package webui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWebInstanceLockRejectsEquivalentHomeAndPreservesMetadata(t *testing.T) {
	parent := t.TempDir()
	home := filepath.Join(parent, "home")
	first, err := acquireWebInstanceLock(home)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := first.writeMetadata("127.0.0.1:3899", "1.2.0")
	if err != nil {
		t.Fatal(err)
	}

	equivalentHome := filepath.Join(home, "..", filepath.Base(home))
	_, err = acquireWebInstanceLock(equivalentHome)
	if !errors.Is(err, errWebUIAlreadyRunning) {
		t.Fatalf("second instance error = %v; want already running", err)
	}
	var runningErr *instanceRunningError
	if !errors.As(err, &runningErr) {
		t.Fatalf("second instance error type = %T", err)
	}
	if runningErr.Metadata.PID != os.Getpid() || runningErr.Metadata.Listen != "127.0.0.1:3899" || runningErr.Metadata.InstanceID != metadata.InstanceID {
		t.Fatalf("unexpected diagnostic metadata: %+v", runningErr.Metadata)
	}

	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := acquireWebInstanceLock(home)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	if err := restarted.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWebInstanceLocksAllowDifferentHomes(t *testing.T) {
	first, err := acquireWebInstanceLock(filepath.Join(t.TempDir(), "first"))
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := acquireWebInstanceLock(filepath.Join(t.TempDir(), "second"))
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
}

func TestServerStartsSchedulerOnlyAfterListenerIsBound(t *testing.T) {
	home := t.TempDir()
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()

	server := newLifecycleTestServer(t, context.Background(), home, occupied.Addr().String(), true)
	if server.scheduler.isActive() {
		t.Fatal("scheduler active before Server.Run")
	}
	err = server.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "bind WebUI listener") {
		t.Fatalf("Server.Run() error = %v; want listener bind error", err)
	}
	if server.scheduler.isActive() {
		t.Fatal("scheduler active after listener bind failure")
	}
}

func TestServerRunRejectsSecondInstanceForSameHome(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	firstListen := availableListen(t)
	first := newLifecycleTestServer(t, ctx, home, firstListen, true)
	firstResult := make(chan error, 1)
	go func() { firstResult <- first.Run(ctx) }()

	metadata := waitForInstanceMetadata(t, filepath.Join(home, InstanceLockFilename), firstListen)
	deadline := time.Now().Add(5 * time.Second)
	for !first.scheduler.isActive() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !first.scheduler.isActive() {
		cancel()
		t.Fatalf("scheduler was not activated after listener bind; metadata=%+v", metadata)
	}

	second := newLifecycleTestServer(t, context.Background(), home, availableListen(t), true)
	err := second.Run(context.Background())
	if !errors.Is(err, errWebUIAlreadyRunning) {
		cancel()
		t.Fatalf("second Server.Run() error = %v; want already running", err)
	}
	if !strings.Contains(err.Error(), firstListen) || !strings.Contains(err.Error(), fmt.Sprintf("pid=%d", os.Getpid())) {
		cancel()
		t.Fatalf("second Server.Run() error lacks diagnostics: %v", err)
	}
	if second.scheduler.isActive() {
		cancel()
		t.Fatal("second instance activated its scheduler")
	}

	cancel()
	select {
	case err := <-firstResult:
		if err != nil {
			t.Fatalf("first Server.Run() shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first Server.Run() did not stop")
	}
	if first.scheduler.isActive() {
		t.Fatal("scheduler remained active after shutdown")
	}

	restarted, err := acquireWebInstanceLock(home)
	if err != nil {
		t.Fatalf("instance lock was not released after shutdown: %v", err)
	}
	if err := restarted.Close(); err != nil {
		t.Fatal(err)
	}
}

func newLifecycleTestServer(t *testing.T, ctx context.Context, home, listen string, schedulerEnabled bool) *Server {
	t.Helper()
	configPath := filepath.Join(home, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.WriteFile(configPath, configForTest(), 0600); err != nil {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	server, err := New(ctx, Options{
		Listen: listen, Home: home,
		ConfigPath: configPath, WebConfig: filepath.Join(home, "webui.yaml"),
		Executable: os.Args[0], Version: "1.2.0", Scheduler: schedulerEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func availableListen(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listen := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return listen
}

func waitForInstanceMetadata(t *testing.T, path, listen string) instanceMetadata {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		metadata, err := readInstanceMetadata(path)
		if err == nil && metadata.Listen == listen && metadata.InstanceID != "" {
			return metadata
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("instance metadata %q was not ready", path)
	return instanceMetadata{}
}
