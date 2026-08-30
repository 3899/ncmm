package ncmm

import (
	"testing"

	"github.com/spf13/cobra"
)

func schedulerMigrationCommand(opts *webOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "web"}
	cmd.Flags().BoolVar(&opts.legacyScheduler, "scheduler", false, "")
	return cmd
}

func TestResolveSchedulerMigration(t *testing.T) {
	t.Run("no migration hint", func(t *testing.T) {
		t.Setenv("NCMM_WEB_PRESERVE_LEGACY_SCHEDULES", "")
		opts := webOptions{}
		cmd := schedulerMigrationCommand(&opts)
		if err := resolveSchedulerMigration(cmd, &opts); err != nil || opts.schedulerMigration != nil {
			t.Fatalf("migration = %+v, %v", opts.schedulerMigration, err)
		}
	})

	t.Run("managed entrypoint hint", func(t *testing.T) {
		t.Setenv("NCMM_WEB_PRESERVE_LEGACY_SCHEDULES", "true")
		opts := webOptions{}
		cmd := schedulerMigrationCommand(&opts)
		if err := resolveSchedulerMigration(cmd, &opts); err != nil || opts.schedulerMigration == nil || !opts.schedulerMigration.PreserveEnabled {
			t.Fatalf("migration = %+v, %v", opts.schedulerMigration, err)
		}
	})

	t.Run("explicit cli before environment", func(t *testing.T) {
		t.Setenv("NCMM_WEB_PRESERVE_LEGACY_SCHEDULES", "true")
		opts := webOptions{}
		cmd := schedulerMigrationCommand(&opts)
		if err := cmd.Flags().Set("scheduler", "false"); err != nil {
			t.Fatal(err)
		}
		if err := resolveSchedulerMigration(cmd, &opts); err != nil || opts.schedulerMigration == nil || opts.schedulerMigration.PreserveEnabled {
			t.Fatalf("migration = %+v, %v", opts.schedulerMigration, err)
		}
	})

	t.Run("invalid environment", func(t *testing.T) {
		t.Setenv("NCMM_WEB_PRESERVE_LEGACY_SCHEDULES", "sometimes")
		opts := webOptions{}
		cmd := schedulerMigrationCommand(&opts)
		if err := resolveSchedulerMigration(cmd, &opts); err == nil {
			t.Fatal("invalid environment value was accepted")
		}
	})
}
