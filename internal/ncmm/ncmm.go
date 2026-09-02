// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package ncmm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/3899/ncmm/config"
	"github.com/3899/ncmm/internal/filelock"
	"github.com/3899/ncmm/pkg/log"
	"github.com/3899/ncmm/pkg/notify"
	"github.com/3899/ncmm/pkg/utils"

	"github.com/spf13/cobra"
)

const title = "                       _    _\n ___  ___  _____  ___ | |_ | |\n|   ||  _||     ||  _||  _|| |\n|_|_||___||_|_|_||___||_|  |_|\n"

type RootOpts struct {
	Debug  bool   // 是否开启命令行debug模式
	Config string // 配置文件路径
	Home   string
}

type Root struct {
	Cfg          *config.Config
	CfgPath      string
	Opts         RootOpts
	cmd          *cobra.Command
	l            *log.Logger
	AppVersion   string
	AppCommit    string
	AppBranch    string
	AppBuildTime string
	// Report collects failures/skips for end-of-run notify summary.
	Report *notify.Report
	// Notifier is nil when notify is disabled or no channels are configured.
	Notifier *notify.Dispatcher
}

func New() *Root {
	c := &Root{
		cmd: &cobra.Command{
			Use:     "ncmm",
			Short:   "ncmm command",
			Long:    "ncmm is a toolbox for netease cloud music\n\nMIT License Copyright (c) 2024 chaunsin\nhttps://github.com/3899/ncmm\n" + title,
			Example: "  ncmm login\n  ncmm playids",
		},
	}
	c.cmd.SetVersionTemplate(`{{printf "%s\n" .Version}}`)
	c.cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Annotations[skipConfigAnnotation] == "true" {
			return nil
		}
		var (
			cfgPath = c.Opts.Config
			home    = filepath.Clean(utils.Ternary(c.Opts.Home != "", c.Opts.Home, config.HomeDir))
		)
		if c.Opts.Config != "" {
			var err error
			if !utils.FileExists(c.Opts.Config) {
				return fmt.Errorf("config file not exists: %s", c.Opts.Config)
			}
			c.CfgPath = c.Opts.Config
			if os.Getenv("NCMM_WEB_CHILD") != "1" {
				if err := config.AutoUpgradeConfigIfNeeded(c.CfgPath); err != nil {
					return fmt.Errorf("upgrade config file error: %w", err)
				}
			}
			c.Cfg, err = loadConfigLocked(c.CfgPath)
			if err != nil {
				return fmt.Errorf("init config error: %s", err)
			}
		} else {
			autoCfgPath := filepath.Join(home, "config.yaml")
			if utils.FileExists(autoCfgPath) {
				var err error
				c.CfgPath = autoCfgPath
				if os.Getenv("NCMM_WEB_CHILD") != "1" {
					if err := config.AutoUpgradeConfigIfNeeded(c.CfgPath); err != nil {
						return fmt.Errorf("upgrade config file error: %w", err)
					}
				}
				c.Cfg, err = loadConfigLocked(autoCfgPath)
				if err != nil {
					return fmt.Errorf("init config error: %s", err)
				}
			} else {
				c.CfgPath = "default"
				c.Cfg = config.GetDefault()
			}
		}

		c.Cfg.ReplaceMagicVariables("HOME", home)
		if err := c.Cfg.Validate(); err != nil {
			return fmt.Errorf("config validate error: %s", err)
		}

		// todo: 暂时关闭debug模式,api中得resty日志需要统一输出本库中得logger里
		c.Cfg.Network.Debug = false
		// 命令行开启了debug模式优先级大于配置文件中得优先级
		if c.Opts.Debug {
			c.Cfg.Log.Stdout = true
			c.Cfg.Log.Level = "debug"
			c.Cfg.Network.Debug = true
		}

		// init logger
		c.l = log.New(c.Cfg.Log)
		log.Default = c.l
		log.Debug("[config] init home=%s path=%s log=%+v network=%+v", home, cfgPath, c.Cfg.Log, c.Cfg.Network)

		c.initNotify()
		if c.Report != nil {
			// Prefer the leaf command path, e.g. "ncmm task" / "ncmm musician sign"
			c.Report.SetCommand(strings.TrimSpace(cmd.CommandPath()))
		}

		if cmd.Name() != "update" {
			c.CheckForUpdatesPreRun()
		}
		return nil
	}
	c.cmd.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Annotations[skipConfigAnnotation] == "true" {
			return nil
		}
		if c.Report != nil {
			c.Report.SetCommand(strings.TrimSpace(cmd.CommandPath()))
		}
		c.FlushNotify(strings.TrimSpace(cmd.CommandPath()))
		c.ShowUpdateNotificationPostRun()
		if c.l != nil {
			return c.l.Close()
		}
		return nil
	}

	c.addFlags()

	// add sub commands
	c.Add(NewLogin(c, c.l).Command())
	c.Add(NewPlayIds(c, c.l).Command())
	c.Add(NewSign(c, c.l).Command())
	m := NewMusician(c, c.l)
	c.Add(m.Command())     // ncmm musician
	c.Add(m.SignCommand()) // ncmm musician-sign
	c.Add(m.VipCommand())  // ncmm musician-vip
	c.Add(NewNote(c, c.l).Command())
	c.Add(NewDailySongShare(c, c.l).Command())
	c.Add(NewVipMemberGift(c, c.l).Command())
	c.Add(NewFansGroup(c, c.l).Command())
	c.Add(NewTask(c, c.l).Command())
	c.Add(newWebCommand(c))
	c.Add(newAuthCommand(c))
	c.Add(newUpdateCommand(c))
	return c
}

func loadConfigLocked(path string) (*config.Config, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	lock, err := filelock.Acquire(ctx, path+".lock")
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	return config.New(path)
}

func (c *Root) addFlags() {
	c.cmd.PersistentFlags().BoolVar(&c.Opts.Debug, "debug", false, "run in debug mode")
	c.cmd.PersistentFlags().StringVarP(&c.Opts.Config, "config", "c", "", "configuration file path")
	c.cmd.PersistentFlags().StringVar(&c.Opts.Home, "home", config.HomeDir, "configuration home path. the home path is used to store running information")
}

func (c *Root) Version(version, buildTime, commitHash, branch string) {
	c.AppVersion = version
	c.AppCommit = commitHash
	c.AppBranch = branch
	c.AppBuildTime = buildTime
	c.cmd.Version = fmt.Sprintf("%s\n Version: \t%s\n Go version: \t%s\n Git branch: \t%s\n Git commit: \t%s\n OS/Arch: \t%s\n Build time: \t%s",
		title, version, runtime.Version(), branch, commitHash, runtime.GOOS+"/"+runtime.GOARCH, buildTime)
}

func (c *Root) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Root) Execute() {
	if err := c.cmd.Execute(); err != nil {
		c.cmd.PrintErrln(err)
		os.Exit(1)
	}
}
