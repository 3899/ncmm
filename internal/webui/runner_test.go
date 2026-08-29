package webui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunManagerHelperProcess(t *testing.T) {
	if len(os.Args) < 2 || os.Args[len(os.Args)-2] != "--run-manager-helper" {
		return
	}
	gate := os.Args[len(os.Args)-1]
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(gate); err == nil {
			os.Exit(0)
		}
		time.Sleep(10 * time.Millisecond)
	}
	os.Exit(2)
}

func newControlledRunManager(t *testing.T, maxParallel int) *runManager {
	t.Helper()
	home := t.TempDir()
	configPath := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(configPath, configForTest(), 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := newRunManager(os.Args[0], configPath, home,
		LogPolicy{RetentionDays: 7, MaxTotalSizeMB: 64},
		ConcurrencyPolicy{MaxParallel: maxParallel},
	)
	if err != nil {
		t.Fatal(err)
	}
	manager.command = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		gate := flagValue(args, "--test-gate")
		return exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunManagerHelperProcess$", "--", "--run-manager-helper", gate)
	}
	return manager
}

func controlledJob(id, policy, account, gate string) Schedule {
	return Schedule{
		ID: id, Name: id, Command: "sign", OverlapPolicy: policy, Source: "test",
		Args: []string{"--cookie-file", account, "--test-gate", gate},
	}
}

func releaseRun(t *testing.T, gate string) {
	t.Helper()
	if err := os.WriteFile(gate, []byte("release"), 0600); err != nil {
		t.Fatal(err)
	}
}

func waitForRunStatus(t *testing.T, manager *runManager, id string, statuses ...string) RunRecord {
	t.Helper()
	wanted := make(map[string]bool, len(statuses))
	for _, status := range statuses {
		wanted[status] = true
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if record, ok := manager.get(id); ok && wanted[record.Status] {
			return record
		}
		time.Sleep(20 * time.Millisecond)
	}
	record, _ := manager.get(id)
	t.Fatalf("run %s status = %q; want one of %v", id, record.Status, statuses)
	return RunRecord{}
}

func TestRunManagerAllowTracksOutOfOrderRuns(t *testing.T) {
	manager := newControlledRunManager(t, 2)
	gateFirst := filepath.Join(manager.home, "first.done")
	gateSecond := filepath.Join(manager.home, "second.done")
	first, err := manager.start(context.Background(), controlledJob("same-job", "allow", "account-a.json", gateFirst))
	if err != nil || first.Status != "running" {
		t.Fatalf("first start = %+v, %v", first, err)
	}
	second, err := manager.start(context.Background(), controlledJob("same-job", "allow", "account-b.json", gateSecond))
	if err != nil || second.Status != "running" {
		t.Fatalf("second start = %+v, %v", second, err)
	}

	releaseRun(t, gateSecond)
	waitForRunStatus(t, manager, second.ID, "success")
	if running, queued := manager.jobActivity("same-job"); !running || queued != 0 {
		t.Fatalf("activity after out-of-order completion = running:%v queued:%d", running, queued)
	}
	releaseRun(t, gateFirst)
	waitForRunStatus(t, manager, first.ID, "success")
	if running, queued := manager.jobActivity("same-job"); running || queued != 0 {
		t.Fatalf("final activity = running:%v queued:%d", running, queued)
	}
}

func TestRunManagerAllowQueuesSameResourceAndPolicySwitch(t *testing.T) {
	manager := newControlledRunManager(t, 2)
	gateFirst := filepath.Join(manager.home, "first.done")
	gateSecond := filepath.Join(manager.home, "second.done")
	first, _ := manager.start(context.Background(), controlledJob("same-job", "skip", "shared.json", gateFirst))
	second, err := manager.start(context.Background(), controlledJob("same-job", "allow", "shared.json", gateSecond))
	if err != nil || first.Status != "running" || second.Status != "queued" {
		t.Fatalf("policy switch states: first=%s second=%s err=%v", first.Status, second.Status, err)
	}
	if running, queued := manager.jobActivity("same-job"); !running || queued != 1 {
		t.Fatalf("policy switch activity = running:%v queued:%d", running, queued)
	}
	releaseRun(t, gateFirst)
	waitForRunStatus(t, manager, second.ID, "running")
	releaseRun(t, gateSecond)
	waitForRunStatus(t, manager, second.ID, "success")
}

func TestRunManagerIncreasingLimitDrainsQueue(t *testing.T) {
	manager := newControlledRunManager(t, 1)
	gateA := filepath.Join(manager.home, "a.done")
	gateB := filepath.Join(manager.home, "b.done")
	a, _ := manager.start(context.Background(), controlledJob("job-a", "skip", "a.json", gateA))
	b, _ := manager.start(context.Background(), controlledJob("job-b", "skip", "b.json", gateB))
	if a.Status != "running" || b.Status != "queued" {
		t.Fatalf("initial states: a=%s b=%s", a.Status, b.Status)
	}
	manager.setConcurrencyPolicy(ConcurrencyPolicy{MaxParallel: 2})
	waitForRunStatus(t, manager, b.ID, "running")
	if manager.runningCount() != 2 {
		t.Fatalf("running count after limit increase = %d; want 2", manager.runningCount())
	}
	releaseRun(t, gateA)
	releaseRun(t, gateB)
	waitForRunStatus(t, manager, a.ID, "success")
	waitForRunStatus(t, manager, b.ID, "success")
}

func TestRunManagerSerializesSharedAccountAndAuditsSkip(t *testing.T) {
	manager := newControlledRunManager(t, 2)
	gateFirst := filepath.Join(manager.home, "first.done")
	gateSecond := filepath.Join(manager.home, "second.done")
	first, err := manager.start(context.Background(), controlledJob("job-a", "skip", "shared.json", gateFirst))
	if err != nil || first.Status != "running" {
		t.Fatalf("first start = %+v, %v", first, err)
	}
	second, err := manager.start(context.Background(), controlledJob("job-b", "skip", "shared.json", gateSecond))
	if err != nil || second.Status != "queued" {
		t.Fatalf("shared-resource start = %+v, %v; want queued", second, err)
	}
	duplicate, err := manager.start(context.Background(), controlledJob("job-a", "skip", "other.json", filepath.Join(manager.home, "duplicate.done")))
	if err != nil || duplicate.Status != "skipped" || !strings.Contains(duplicate.Error, "已有活动运行") {
		t.Fatalf("duplicate start = %+v, %v; want audited skip", duplicate, err)
	}
	if logData, err := manager.readLog(duplicate.ID); err != nil || !strings.Contains(string(logData), "skipped") {
		t.Fatalf("skip log = %q, %v", logData, err)
	}

	releaseRun(t, gateFirst)
	waitForRunStatus(t, manager, first.ID, "success")
	waitForRunStatus(t, manager, second.ID, "running")
	releaseRun(t, gateSecond)
	waitForRunStatus(t, manager, second.ID, "success")
}

func TestRunManagerParallelAccountsAndDatabaseMutex(t *testing.T) {
	t.Run("different accounts", func(t *testing.T) {
		manager := newControlledRunManager(t, 2)
		gateA := filepath.Join(manager.home, "a.done")
		gateB := filepath.Join(manager.home, "b.done")
		a, _ := manager.start(context.Background(), controlledJob("job-a", "skip", "a.json", gateA))
		b, _ := manager.start(context.Background(), controlledJob("job-b", "skip", "b.json", gateB))
		if a.Status != "running" || b.Status != "running" || manager.runningCount() != 2 {
			t.Fatalf("different accounts did not run in parallel: a=%s b=%s count=%d", a.Status, b.Status, manager.runningCount())
		}
		releaseRun(t, gateA)
		releaseRun(t, gateB)
		waitForRunStatus(t, manager, a.ID, "success")
		waitForRunStatus(t, manager, b.ID, "success")
	})

	t.Run("shared database", func(t *testing.T) {
		manager := newControlledRunManager(t, 2)
		gateA := filepath.Join(manager.home, "a.done")
		gateB := filepath.Join(manager.home, "b.done")
		jobA := controlledJob("job-a", "skip", "a.json", gateA)
		jobB := controlledJob("job-b", "skip", "b.json", gateB)
		jobA.Command = "playids"
		jobB.Command = "playids"
		a, _ := manager.start(context.Background(), jobA)
		b, _ := manager.start(context.Background(), jobB)
		if a.Status != "running" || b.Status != "queued" {
			t.Fatalf("database mutex states: a=%s b=%s", a.Status, b.Status)
		}
		releaseRun(t, gateA)
		waitForRunStatus(t, manager, b.ID, "running")
		releaseRun(t, gateB)
		waitForRunStatus(t, manager, b.ID, "success")
	})
}

func TestRunManagerStopsQueuedRun(t *testing.T) {
	manager := newControlledRunManager(t, 1)
	gateFirst := filepath.Join(manager.home, "first.done")
	first, _ := manager.start(context.Background(), controlledJob("job-a", "skip", "a.json", gateFirst))
	queued, _ := manager.start(context.Background(), controlledJob("job-b", "skip", "b.json", filepath.Join(manager.home, "second.done")))
	if queued.Status != "queued" {
		t.Fatalf("second status = %s; want queued", queued.Status)
	}
	if err := manager.stop(queued.ID); err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, manager, queued.ID, "stopped")
	if _, count := manager.jobActivity("job-b"); count != 0 {
		t.Fatalf("stopped queue still active: %d", count)
	}
	releaseRun(t, gateFirst)
	waitForRunStatus(t, manager, first.ID, "success")
}
