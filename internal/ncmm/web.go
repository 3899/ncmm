package ncmm

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/3899/ncmm/config"
	"github.com/3899/ncmm/internal/atomicfile"
	"github.com/3899/ncmm/internal/filelock"
	"github.com/3899/ncmm/internal/webui"
	"github.com/3899/ncmm/pkg/utils"
	"github.com/spf13/cobra"
)

type webOptions struct {
	listen             string
	webConfig          string
	legacyScheduler    bool
	schedulerMigration *webui.SchedulerMigration
	background         bool
	secureCookie       bool
}

func newWebCommand(root *Root) *cobra.Command {
	secureCookie, _ := strconv.ParseBool(os.Getenv("NCMM_WEB_SECURE_COOKIE"))
	opts := webOptions{secureCookie: secureCookie}
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start the ncmm management service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := resolveSchedulerMigration(cmd, &opts); err != nil {
				return err
			}
			if opts.background {
				return startWebBackground(opts.listen, webAuthHome(root))
			}
			return runWeb(cmd.Context(), root, opts)
		},
	}
	cmd.Flags().StringVar(&opts.listen, "listen", "127.0.0.1:3899", "WebUI listen address")
	cmd.Flags().StringVar(&opts.webConfig, "web-config", "", "WebUI settings file path")
	cmd.Flags().BoolVar(&opts.legacyScheduler, "scheduler", false, "preserve enabled schedules while migrating a v1 WebUI configuration")
	_ = cmd.Flags().MarkHidden("scheduler")
	cmd.Flags().BoolVar(&opts.background, "background", false, "start the WebUI without a console window (Windows only)")
	cmd.Flags().BoolVar(&opts.secureCookie, "secure-cookie", secureCookie, "mark the WebUI session cookie Secure (or NCMM_WEB_SECURE_COOKIE)")
	cmd.AddCommand(newWebStatusCommand(root), newWebStopCommand(root))
	return cmd
}

func resolveSchedulerMigration(cmd *cobra.Command, opts *webOptions) error {
	opts.schedulerMigration = nil
	if cmd.Flags().Changed("scheduler") {
		opts.schedulerMigration = &webui.SchedulerMigration{
			PreserveEnabled: opts.legacyScheduler,
		}
		return nil
	}
	if value := os.Getenv("NCMM_WEB_PRESERVE_LEGACY_SCHEDULES"); value != "" {
		preserve, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid NCMM_WEB_PRESERVE_LEGACY_SCHEDULES value %q: %w", value, err)
		}
		opts.schedulerMigration = &webui.SchedulerMigration{
			PreserveEnabled: preserve,
		}
	}
	return nil
}

func runWeb(parent context.Context, root *Root, opts webOptions) error {
	home := filepath.Clean(utils.Ternary(root.Opts.Home != "", root.Opts.Home, config.HomeDir))
	if err := os.MkdirAll(home, 0755); err != nil {
		return err
	}
	configPath := root.CfgPath
	if configPath == "" || configPath == "default" {
		configPath = filepath.Join(home, "config.yaml")
		if !utils.FileExists(configPath) {
			ctx, cancel := context.WithTimeout(parent, 10*time.Second)
			lock, lockErr := filelock.Acquire(ctx, configPath+".lock")
			cancel()
			if lockErr != nil {
				return fmt.Errorf("initialize config lock: %w", lockErr)
			}
			if !utils.FileExists(configPath) {
				if err := atomicfile.Write(configPath, config.DefaultYAML(), 0600); err != nil {
					_ = lock.Close()
					return fmt.Errorf("initialize config: %w", err)
				}
			}
			if err := lock.Close(); err != nil {
				return fmt.Errorf("release config lock: %w", err)
			}
		}
	}
	configPath, _ = filepath.Abs(configPath)
	if opts.webConfig == "" {
		opts.webConfig = filepath.Join(home, "webui.yaml")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	server, err := webui.New(ctx, webui.Options{
		Listen: opts.listen, Home: home, ConfigPath: configPath,
		WebConfig: opts.webConfig, Executable: executable, Version: root.AppVersion,
		SecureCookie: opts.secureCookie, Output: root.cmd.Printf,
		SchedulerMigration: opts.schedulerMigration,
	})
	if err != nil {
		return err
	}
	return server.Run(ctx)
}
