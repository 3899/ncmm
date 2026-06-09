// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package ncmm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/3899/ncmm/config"
	"github.com/3899/ncmm/pkg/log"

	"github.com/spf13/cobra"
)

type Login struct {
	root   *Root
	cmd    *cobra.Command
	l      *log.Logger
	isMain bool
}

func NewLogin(root *Root, l *log.Logger) *Login {
	c := &Login{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:     "login",
			Short:   "Login netease cloud music",
			Example: "  ncmm login -h\n  ncmm login qrcode\n  ncmm login phone\n  ncmm login cookiecloud\n  ncmm login cookie",
		},
	}
	c.addFlags()
	c.Add(qrcode(c, l))
	c.Add(phone(c, l))
	c.Add(cookieCloud(c, l))
	c.Add(cookie(c, l))

	return c
}

func (c *Login) addFlags() {
	c.cmd.PersistentFlags().BoolVarP(&c.isMain, "main", "m", false, "是否作为主账号登录 (默认登录为辅助账号)")
}

func (c *Login) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Login) Command() *cobra.Command {
	return c.cmd
}

// saveLoginResult 处理登录 Cookie 的保存，根据主/辅账号智能推导输出文件名，并回写带有昵称注释的账号配置到 config.yaml
func (c *Login) saveLoginResult(ctx context.Context, nickname string, uid int64, tempCookieFile string, customOutput string, rawInputName string) error {
	var targetDir string
	if c.root.Cfg != nil && c.root.Cfg.Network != nil && c.root.Cfg.Network.Cookie.Filepath != "" {
		targetDir = filepath.Dir(c.root.Cfg.Network.Cookie.Filepath)
	}
	if targetDir == "" {
		home := c.root.Opts.Home
		if home == "" {
			home = config.HomeDir
		}
		home = filepath.Clean(home)
		if home == filepath.Clean(config.HomeDir) {
			home = filepath.Join(home, ".ncmm")
		}
		targetDir = home
	}

	var finalPath string
	if c.isMain {
		if c.root.Cfg.Accounts != nil && c.root.Cfg.Accounts.Main != "" {
			finalPath = c.root.Cfg.Accounts.Main
		} else {
			finalPath = filepath.Join(targetDir, "cookie.json")
		}
	} else {
		var filename string
		if customOutput != "" {
			filename = customOutput
		} else if rawInputName != "" {
			base := filepath.Base(rawInputName)
			ext := filepath.Ext(base)
			nameWithoutExt := strings.TrimSuffix(base, ext)
			filename = nameWithoutExt + ".json"
		} else {
			filename = fmt.Sprintf("fan_%d.json", uid)
		}

		if filepath.IsAbs(filename) {
			finalPath = filename
		} else {
			finalPath = filepath.Join(targetDir, filename)
		}
	}
	finalPath = filepath.Clean(finalPath)

	// 如果临时 Cookie 文件存在且不等于最终路径，则将其移动/重命名过去
	if tempCookieFile != "" && tempCookieFile != finalPath {
		if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
			return fmt.Errorf("创建 Cookie 目录失败: %w", err)
		}
		if err := os.Rename(tempCookieFile, finalPath); err != nil {
			// 跨盘重命名失败兜底：复制 + 删除
			data, errRead := os.ReadFile(tempCookieFile)
			if errRead != nil {
				return fmt.Errorf("读取临时 Cookie 失败: %w", errRead)
			}
			if errWrite := os.WriteFile(finalPath, data, 0644); errWrite != nil {
				return fmt.Errorf("写入最终 Cookie 失败: %w", errWrite)
			}
			_ = os.Remove(tempCookieFile)
		}
	}

	// 规范化显示路径为包含 ${HOME} 的相对路径形式（若位于 Home 目录下）
	finalPathDisplay := formatHomePath(finalPath)

	if c.root.CfgPath == "" || c.root.CfgPath == "default" {
		c.cmd.Printf("未检测到本地配置文件，跳过配置自动回写。Cookie 已保存至: %s\n", finalPath)
		return nil
	}

	if c.root.Cfg.Accounts == nil {
		c.root.Cfg.Accounts = &config.AccountsConf{}
	}

	var mainPath, mainNickname string
	var secondaryPaths []string
	var secondaryNicknames []string

	if c.isMain {
		mainPath = finalPathDisplay
		mainNickname = nickname
		secondaryPaths = make([]string, len(c.root.Cfg.Accounts.Secondary))
		for i, sec := range c.root.Cfg.Accounts.Secondary {
			secondaryPaths[i] = formatHomePath(sec)
		}
	} else {
		mainPath = formatHomePath(c.root.Cfg.Accounts.Main)
		secondaryPaths = make([]string, 0, len(c.root.Cfg.Accounts.Secondary)+1)
		found := false
		for _, sec := range c.root.Cfg.Accounts.Secondary {
			secFmt := formatHomePath(sec)
			if secFmt == finalPathDisplay {
				found = true
			}
			secondaryPaths = append(secondaryPaths, secFmt)
		}
		if !found {
			secondaryPaths = append(secondaryPaths, finalPathDisplay)
		}
	}

	// 传递我们此次登录生成的昵称注释。非本账号的既有注释将在 config.UpdateAccountsInFile 中被完美还原。
	if !c.isMain {
		secondaryNicknames = make([]string, len(secondaryPaths))
		for i, path := range secondaryPaths {
			if path == finalPathDisplay {
				secondaryNicknames[i] = nickname
			}
		}
	}

	// 更新内存配置
	c.root.Cfg.Accounts.Main = mainPath
	c.root.Cfg.Accounts.Secondary = secondaryPaths

	// 更新配置文件
	err := config.UpdateAccountsInFile(c.root.CfgPath, mainPath, mainNickname, secondaryPaths, secondaryNicknames)
	if err != nil {
		return fmt.Errorf("回写配置文件失败: %w", err)
	}

	c.cmd.Printf("账号登录配置已更新成功！\nCookie 文件: %s\n", finalPath)
	return nil
}

func formatHomePath(path string) string {
	if path == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.ToSlash(path)
	}
	home = filepath.Clean(home)
	cleanedPath := filepath.Clean(path)
	if strings.HasPrefix(cleanedPath, home) {
		rel, err := filepath.Rel(home, cleanedPath)
		if err == nil {
			return "${HOME}/" + filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(path)
}
