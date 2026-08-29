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
	listen       string
	webConfig    string
	scheduler    bool
	background   bool
	secureCookie bool
}

func newWebCommand(root *Root) *cobra.Command {
	secureCookie, _ := strconv.ParseBool(os.Getenv("NCMM_WEB_SECURE_COOKIE"))
	opts := webOptions{secureCookie: secureCookie}
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start the optional ncmm WebUI",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.background {
				return startWebBackground(opts.listen)
			}
			return runWeb(cmd.Context(), root, opts)
		},
	}
	cmd.Flags().StringVar(&opts.listen, "listen", "127.0.0.1:3899", "WebUI listen address")
	cmd.Flags().StringVar(&opts.webConfig, "web-config", "", "WebUI settings file path")
	cmd.Flags().BoolVar(&opts.scheduler, "scheduler", false, "enable the built-in scheduler")
	cmd.Flags().BoolVar(&opts.background, "background", false, "start the WebUI without a console window (Windows only)")
	cmd.Flags().BoolVar(&opts.secureCookie, "secure-cookie", secureCookie, "mark the WebUI session cookie Secure (or NCMM_WEB_SECURE_COOKIE)")
	return cmd
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
		Scheduler: opts.scheduler, SecureCookie: opts.secureCookie, Output: root.cmd.Printf,
	})
	if err != nil {
		return err
	}
	return server.Run(ctx)
}
