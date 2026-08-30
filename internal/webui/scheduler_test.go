package webui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSchedulerUsesPerJobEnabledStateAndAllowsManualRun(t *testing.T) {
	manager := newControlledRunManager(t, 1)
	defer manager.supervisor.Close(0)
	store, err := newWebConfigStore(filepath.Join(manager.home, "webui.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	gate := filepath.Join(manager.home, "manual.done")
	job := controlledJob("daily", "skip", "account.json", gate)
	job.Enabled = true
	job.Cron = "30 8 * * *"
	job, err = store.upsert(job)
	if err != nil {
		t.Fatal(err)
	}
	scheduler, err := newScheduler(context.Background(), store, manager)
	if err != nil {
		t.Fatal(err)
	}
	scheduler.start()
	defer scheduler.close()
	if len(scheduler.cron.Entries()) != 1 {
		t.Fatalf("enabled scheduler entries = %d", len(scheduler.cron.Entries()))
	}
	if jobs := scheduler.list(); len(jobs) != 1 || len(jobs[0].NextRuns) != 5 {
		t.Fatalf("enabled schedule next runs = %+v", jobs)
	}

	job.Enabled = false
	if _, err := scheduler.upsert(job); err != nil {
		t.Fatal(err)
	}
	if len(scheduler.cron.Entries()) != 0 {
		t.Fatalf("disabled schedule entries = %d", len(scheduler.cron.Entries()))
	}
	if jobs := scheduler.list(); len(jobs) != 1 || len(jobs[0].NextRuns) != 0 {
		t.Fatalf("disabled schedule still has automatic next runs: %+v", jobs)
	}

	record, err := scheduler.runNow(job.ID)
	if err != nil || record.Status != "running" {
		t.Fatalf("manual run for disabled schedule = %+v, %v", record, err)
	}
	releaseRun(t, gate)
	waitForRunStatus(t, manager, record.ID, "success")
}

func TestSchedulerRejectsInvalidPersistedConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*testing.T, *webConfigStore)
		want    string
	}{
		{
			name: "timezone",
			prepare: func(t *testing.T, store *webConfigStore) {
				cfg := store.snapshot()
				if _, err := store.updateSettings("Mars/Olympus", cfg.Logs, cfg.Concurrency); err != nil {
					t.Fatal(err)
				}
			},
			want: "invalid timezone",
		},
		{
			name: "cron",
			prepare: func(t *testing.T, store *webConfigStore) {
				if _, err := store.upsert(Schedule{ID: "bad", Name: "Bad", Enabled: true, Cron: "not-a-cron", Command: "task"}); err != nil {
					t.Fatal(err)
				}
			},
			want: "invalid cron expression",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, err := newWebConfigStore(filepath.Join(t.TempDir(), "webui.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			test.prepare(t, store)
			if _, err := newScheduler(context.Background(), store, nil); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("newScheduler() error = %v; want %q", err, test.want)
			}
		})
	}
}

func TestSchedulerLoadsLegacyEnvironmentRulesReadOnly(t *testing.T) {
	t.Setenv("CRON_NCMM_VALID", "15 9 * * * sign --cookie-file fan.json")
	t.Setenv("CRON_NCMM_INVALID", "invalid")
	manager := newControlledRunManager(t, 1)
	defer manager.supervisor.Close(0)
	store, err := newWebConfigStore(filepath.Join(manager.home, "webui.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	scheduler, err := newScheduler(context.Background(), store, manager)
	if err != nil {
		t.Fatal(err)
	}
	var found *ScheduleView
	for _, job := range scheduler.list() {
		if job.Name == "CRON_NCMM_VALID" {
			copyJob := job
			found = &copyJob
		}
		if job.Name == "CRON_NCMM_INVALID" {
			t.Fatal("invalid environment rule was loaded")
		}
	}
	if found == nil || !found.ReadOnly || found.Source != "environment" || found.Command != "sign" || len(found.Args) != 2 || found.Args[1] != "fan.json" {
		t.Fatalf("environment schedule = %+v", found)
	}
	if err := scheduler.delete(found.ID); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("delete environment schedule error = %v", err)
	}
	_ = os.Unsetenv("CRON_NCMM_VALID")
}
