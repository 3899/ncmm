// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package ncmm

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/3899/ncmm/config"
	"github.com/3899/ncmm/internal/webui"
	"github.com/3899/ncmm/pkg/log"
	"github.com/3899/ncmm/pkg/notify"
	"github.com/3899/ncmm/pkg/utils"

	"github.com/spf13/cobra"
)

type Plugin struct {
	root *Root
	l    *log.Logger
	cmd  *cobra.Command
}

type PluginParamSchema = webui.PluginParamSchema
type PluginManifest = webui.PluginManifest
type PluginInfo = webui.PluginInfo

func DefaultWithdrawManifest() PluginManifest {
	return PluginManifest{
		Name:            "withdraw",
		Title:           "音乐人收益提现",
		Version:         "1.0.0",
		Description:     "网易云音乐人金币/收益自动提现与官方放量整点毫秒级极速抢额度助手",
		Author:          "3899",
		RecommendedCron: "30 59 7,11,17,19,23 * * *",
		Params: []PluginParamSchema{
			{
				Name:        "target",
				Label:       "目标金额 (元)",
				Type:        "chips",
				Options:     []interface{}{"15", "25", "0.3", "0.1"},
				Default:     "15,25,0.3,0.1",
				Description: "优先尝试提现的金额档位列表（逗号分隔，按从前到后优先级尝试）",
			},
			{
				Name:  "snipe",
				Label: "抢购时段",
				Type:  "select",
				Options: []interface{}{
					map[string]string{"label": "全天自动轮巡 (0/8/12/18/20点)", "value": "auto"},
					map[string]string{"label": "00:00:00 场次", "value": "00:00:00"},
					map[string]string{"label": "08:00:00 场次", "value": "08:00:00"},
					map[string]string{"label": "12:00:00 场次", "value": "12:00:00"},
					map[string]string{"label": "18:00:00 场次", "value": "18:00:00"},
					map[string]string{"label": "20:00:00 场次", "value": "20:00:00"},
					map[string]string{"label": "立即提现 (不等待整点)", "value": ""},
				},
				Default:     "auto",
				Description: "选择整点秒杀模式或立即发起提现",
			},
			{
				Name:        "advance",
				Label:       "提前请求 (ms)",
				Type:        "number",
				Default:     50,
				Description: "整点抢额度提前发包毫秒数，建议 30-80ms",
			},
			{
				Name:  "channel",
				Label: "提现渠道",
				Type:  "select",
				Options: []interface{}{
					map[string]string{"label": "支付宝 (ALIPAY)", "value": "ALIPAY"},
					map[string]string{"label": "银行卡 (BANK_CARD)", "value": "BANK_CARD"},
				},
				Default: "ALIPAY",
			},
			{
				Name:        "fallback",
				Label:       "大额不足降级小额",
				Type:        "boolean",
				Default:     true,
				Description: "首选档位无库存时自动平滑降级尝试有库存的其他档位",
			},
			{
				Name:        "account",
				Label:       "执行账号",
				Type:        "account-select",
				Description: "从系统已导入的账号中直接选择",
			},
		},
	}
}

type PluginContext struct {
	Version    string         `json:"version"`
	Command    string         `json:"command"`
	Home       string         `json:"home"`
	ConfigPath string         `json:"config_path"`
	NotifyFile string         `json:"notify_file,omitempty"`
	Account    *PluginAccount `json:"account,omitempty"`
}

type PluginAccount struct {
	Filepath string `json:"filepath"`
	IsMain   bool   `json:"is_main"`
}

type PluginResult struct {
	Success bool          `json:"success"`
	Message string        `json:"message"`
	Data    interface{}   `json:"data,omitempty"`
	Notify  *PluginNotify `json:"notify,omitempty"`
}

type PluginNotify struct {
	ShouldNotify bool   `json:"should_notify"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	Level        string `json:"level"`
}

func NewPlugin(root *Root, l *log.Logger) *Plugin {
	p := &Plugin{root: root, l: l}
	p.cmd = &cobra.Command{
		Use:     "plugin",
		Short:   "Manage and execute external ncmm plugins",
		Example: "  ncmm plugin list\n  ncmm plugin run demo\n  ncmm plugin run demo -- --flag1 value",
	}
	p.cmd.AddCommand(p.listCommand())
	p.cmd.AddCommand(p.runCommand())
	p.cmd.AddCommand(p.installCommand())
	return p
}

func (p *Plugin) Command() *cobra.Command {
	return p.cmd
}

// GetPluginSearchDirs returns search paths for plugins.
func (p *Plugin) GetPluginSearchDirs() []string {
	var dirs []string
	seen := make(map[string]bool)

	addDir := func(d string) {
		d = filepath.Clean(d)
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}

	// 1. Current working directory / plugins
	addDir("plugins")
	// 2. Config Home / plugins
	home := config.HomeDir
	if p.root != nil && p.root.Opts.Home != "" {
		home = p.root.Opts.Home
	}
	addDir(filepath.Join(home, "plugins"))
	// 3. Executable directory / plugins
	if exe, err := os.Executable(); err == nil {
		addDir(filepath.Join(filepath.Dir(exe), "plugins"))
	}

	return dirs
}

// DiscoverPlugins scans configured directories for executable plugins.
func (p *Plugin) DiscoverPlugins() []PluginInfo {
	var list []PluginInfo
	seenNames := make(map[string]bool)

	dirs := p.GetPluginSearchDirs()
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			fullPath := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				// Check sub-directory (e.g. plugins/ncmm-demo/ncmm-demo[.exe])
				subEntries, err := os.ReadDir(fullPath)
				if err != nil {
					continue
				}
				for _, sub := range subEntries {
					if sub.IsDir() {
						continue
					}
					subName := sub.Name()
					if isExecutableFile(subName) {
						pluginName := parsePluginName(subName)
						if pluginName != "" && !seenNames[pluginName] {
							seenNames[pluginName] = true
							list = append(list, p.loadPluginInfo(pluginName, filepath.Join(fullPath, subName)))
						}
					}
				}
			} else {
				// Flat file in plugins directory
				name := entry.Name()
				if isExecutableFile(name) {
					pluginName := parsePluginName(name)
					if pluginName != "" && !seenNames[pluginName] {
						seenNames[pluginName] = true
						list = append(list, p.loadPluginInfo(pluginName, fullPath))
					}
				}
			}
		}
	}

	return list
}

func (p *Plugin) loadPluginInfo(name, path string) PluginInfo {
	info := PluginInfo{
		Name: name,
		Path: path,
	}
	dir := filepath.Dir(path)
	manifestPath := filepath.Join(dir, "plugin.json")
	if !utils.FileExists(manifestPath) {
		parentManifest := filepath.Join(filepath.Dir(dir), "plugin.json")
		if utils.FileExists(parentManifest) {
			manifestPath = parentManifest
		}
	}
	if utils.FileExists(manifestPath) {
		if data, err := os.ReadFile(manifestPath); err == nil {
			var m PluginManifest
			if json.Unmarshal(data, &m) == nil {
				if m.Title != "" {
					info.Title = m.Title
				}
				if m.Version != "" {
					info.Version = m.Version
				}
				if m.Description != "" {
					info.Description = m.Description
				}
				if m.Author != "" {
					info.Author = m.Author
				}
				if m.RecommendedCron != "" {
					info.RecommendedCron = m.RecommendedCron
				}
				if len(m.Params) > 0 {
					info.Params = m.Params
				}
			}
		}
	}
	if info.Name == "withdraw" && len(info.Params) == 0 {
		defaultM := DefaultWithdrawManifest()
		if info.Title == "" {
			info.Title = defaultM.Title
		}
		if info.Version == "" {
			info.Version = defaultM.Version
		}
		if info.Description == "" {
			info.Description = defaultM.Description
		}
		if info.Author == "" {
			info.Author = defaultM.Author
		}
		if info.RecommendedCron == "" {
			info.RecommendedCron = defaultM.RecommendedCron
		}
		info.Params = defaultM.Params
	}
	if info.Title == "" {
		info.Title = info.Name
	}
	return info
}

func (p *Plugin) UninstallPlugin(name string) error {
	name = strings.TrimSpace(name)
	pluginPath, err := p.FindPlugin(name)
	if err != nil {
		return err
	}
	dir := filepath.Dir(pluginPath)
	base := filepath.Base(dir)
	// If it's located in a plugin-specific folder like plugins/ncmm-withdraw or plugins/withdraw
	if strings.HasPrefix(strings.ToLower(base), "ncmm-") || strings.EqualFold(base, name) {
		return os.RemoveAll(dir)
	}
	return os.Remove(pluginPath)
}

func isExecutableFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	if runtime.GOOS == "windows" {
		return ext == ".exe" || ext == ".bat" || ext == ".cmd" || ext == ".py"
	}
	// On Unix, ignore source/doc extensions
	if ext == ".go" || ext == ".md" || ext == ".txt" || ext == ".json" || ext == ".yaml" || ext == ".yml" {
		return false
	}
	return true
}

func parsePluginName(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	// Remove prefix ncmm- or ncmm_
	if strings.HasPrefix(stem, "ncmm-") {
		stem = strings.TrimPrefix(stem, "ncmm-")
	} else if strings.HasPrefix(stem, "ncmm_") {
		stem = strings.TrimPrefix(stem, "ncmm_")
	}

	stem = strings.TrimSpace(stem)
	return stem
}

// FindPlugin locates an executable plugin by its registered name.
func (p *Plugin) FindPlugin(name string) (string, error) {
	name = strings.TrimSpace(name)
	plugins := p.DiscoverPlugins()
	for _, plug := range plugins {
		if strings.EqualFold(plug.Name, name) {
			return plug.Path, nil
		}
	}
	return "", fmt.Errorf("plugin %q not found in search paths: %v", name, p.GetPluginSearchDirs())
}

// ExecutePlugin executes a plugin with PluginContext and captures the result.
func (p *Plugin) ExecutePlugin(ctx context.Context, pluginPath string, pctx *PluginContext, extraArgs ...string) (*PluginResult, error) {
	absPluginPath, err := filepath.Abs(pluginPath)
	if err != nil {
		absPluginPath = pluginPath
	}

	cmd := exec.CommandContext(ctx, absPluginPath, extraArgs...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env,
		"NCMM_PLUGIN=1",
		"NCMM_HOME="+pctx.Home,
		"NCMM_CONFIG_PATH="+pctx.ConfigPath,
		"NCMM_NOTIFY_FILE="+pctx.NotifyFile,
	)
	if pctx.Account != nil {
		cmd.Env = append(cmd.Env,
			"NCMM_ACCOUNT_COOKIE="+pctx.Account.Filepath,
			fmt.Sprintf("NCMM_ACCOUNT_IS_MAIN=%t", pctx.Account.IsMain),
		)
	}

	inputBytes, _ := json.Marshal(pctx)
	cmd.Stdin = bytes.NewReader(inputBytes)

	var stdoutBuf, stderrBuf bytes.Buffer
	// Duplicate stdout so user sees progress while we capture the JSON result
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("plugin execution failed: %w (stderr: %s)", err, stderrBuf.String())
	}

	// Parse JSON output if present
	var result PluginResult
	lines := strings.Split(stdoutBuf.String(), "\n")
	foundJSON := false
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
			if jsonErr := json.Unmarshal([]byte(line), &result); jsonErr == nil && (result.Success || result.Message != "") {
				foundJSON = true
				break
			}
		}
	}

	if !foundJSON {
		result.Success = true
		result.Message = "executed successfully"
	}

	// Dispatch notification if requested by the plugin
	if result.Notify != nil && result.Notify.ShouldNotify {
		p.dispatchPluginNotification(ctx, result.Notify)
	}

	return &result, nil
}

func (p *Plugin) dispatchPluginNotification(ctx context.Context, n *PluginNotify) {
	if p.root == nil || p.root.Notifier == nil || p.root.Notifier.Len() == 0 {
		return
	}
	level := n.Level
	if level == "" {
		level = "info"
	}
	msg := notify.Message{
		Title:   n.Title,
		Content: n.Content,
		Level:   level,
	}
	if err := p.root.Notifier.SendAll(ctx, msg); err != nil {
		log.Warn("[plugin-notify] failed to send notification: %v", err)
	} else {
		log.Info("[plugin-notify] notification sent: %s", n.Title)
	}
}

func (p *Plugin) listCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all discovered plugins",
		Run: func(cmd *cobra.Command, args []string) {
			plugins := p.DiscoverPlugins()
			if len(plugins) == 0 {
				cmd.Println("No plugins found in search paths:")
				for _, dir := range p.GetPluginSearchDirs() {
					cmd.Printf("  - %s\n", dir)
				}
				cmd.Println("\nTo install a plugin, drop its executable into one of the directories above.")
				return
			}

			cmd.Printf("Discovered %d plugin(s):\n", len(plugins))
			for i, plug := range plugins {
				cmd.Printf("  [%d] %-15s -> %s\n", i+1, plug.Name, plug.Path)
			}
		},
	}
}

func (p *Plugin) runCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "run <plugin-name> [-- extra args...]",
		Short:              "Run a specific plugin with forwarded arguments",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
				return cmd.Help()
			}

			pluginName := args[0]
			pluginPath, err := p.FindPlugin(pluginName)
			if err != nil {
				return err
			}

			// Extract forwarded arguments and detect optional account file
			var accountFile string
			var extraArgs []string
			rawArgs := args[1:]
			for i := 0; i < len(rawArgs); i++ {
				arg := rawArgs[i]
				if i == 0 && arg == "--" {
					continue
				}
				if arg == "-a" || arg == "--account" {
					if i+1 < len(rawArgs) {
						accountFile = rawArgs[i+1]
					}
				} else if strings.HasPrefix(arg, "--account=") {
					accountFile = strings.TrimPrefix(arg, "--account=")
				}
				extraArgs = append(extraArgs, arg)
			}

			home := utils.Ternary(p.root.Opts.Home != "", p.root.Opts.Home, config.HomeDir)
			pctx := &PluginContext{
				Version:    p.root.AppVersion,
				Command:    "plugin run",
				Home:       home,
				ConfigPath: p.root.CfgPath,
			}
			if p.root.Cfg != nil && p.root.Cfg.Notify != nil {
				pctx.NotifyFile = p.root.Cfg.Notify.File
			}

			// If specific account passed or main account exists
			if accountFile != "" {
				pctx.Account = &PluginAccount{Filepath: accountFile, IsMain: true}
			} else if p.root.Cfg != nil && p.root.Cfg.Accounts != nil && p.root.Cfg.Accounts.Main != "" {
				pctx.Account = &PluginAccount{Filepath: p.root.Cfg.Accounts.Main, IsMain: true}
			}

			cmd.Printf("Running plugin [%s] (%s)...\n", pluginName, pluginPath)
			res, err := p.ExecutePlugin(cmd.Context(), pluginPath, pctx, extraArgs...)
			if err != nil {
				return err
			}
			if !res.Success {
				return fmt.Errorf("plugin reported error: %s", res.Message)
			}
			cmd.Printf("Plugin [%s] completed: %s\n", pluginName, res.Message)
			return nil
		},
	}

	return cmd
}

func (p *Plugin) installCommand() *cobra.Command {
	var token string
	var destDir string

	cmd := &cobra.Command{
		Use:   "install <url-or-github-repo>",
		Short: "Install a plugin from a remote URL or GitHub repository",
		Long: `Download and install a plugin binary into the local plugins/ directory.

Examples:
  # Install from direct archive or binary URL:
  ncmm plugin install https://example.com/ncmm-demo_Linux_x86_64.tar.gz

  # Install from private or public GitHub repository using a personal access token:
  ncmm plugin install your-username/ncmm-demo --token ghp_xxxx
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := strings.TrimSpace(args[0])
			if destDir == "" {
				home := utils.Ternary(p.root.Opts.Home != "", p.root.Opts.Home, config.HomeDir)
				destDir = filepath.Join(home, "plugins")
			}
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
				_, err := p.InstallFromURL(cmd.Context(), target, token, destDir, cmd.Printf)
				return err
			}
			if strings.Contains(target, "/") {
				_, err := p.InstallFromGitHub(cmd.Context(), target, token, destDir, cmd.Printf)
				return err
			}
			return fmt.Errorf("invalid install target %q: expected URL or 'owner/repo'", target)
		},
	}

	cmd.Flags().StringVarP(&token, "token", "t", "", "GitHub Personal Access Token (for private repositories)")
	cmd.Flags().StringVarP(&destDir, "dir", "d", "", "target plugins directory (default: {home}/plugins)")
	return cmd
}

func (p *Plugin) DefaultPluginDir() string {
	home := config.HomeDir
	if p.root != nil && p.root.Opts.Home != "" {
		home = p.root.Opts.Home
	}
	return filepath.Join(home, "plugins")
}

func (p *Plugin) InstallFromURL(ctx context.Context, targetURL, token, destDir string, logFn func(string, ...any)) (string, error) {
	if logFn == nil {
		logFn = func(string, ...any) {}
	}
	if destDir == "" {
		destDir = p.DefaultPluginDir()
	}
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return "", err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	logFn("Downloading %s...\n", targetURL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	base := filepath.Base(targetURL)
	if idx := strings.Index(base, "?"); idx != -1 {
		base = base[:idx]
	}

	installedName, err := p.InstallFromArchive(data, base, destDir)
	if err != nil {
		return "", fmt.Errorf("extract plugin: %w", err)
	}

	logFn("✅ Plugin [%s] installed successfully into %s!\n", installedName, destDir)
	return installedName, nil
}

func (p *Plugin) InstallFromGitHub(ctx context.Context, repo, token, destDir string, logFn func(string, ...any)) (string, error) {
	if logFn == nil {
		logFn = func(string, ...any) {}
	}
	if destDir == "" {
		destDir = p.DefaultPluginDir()
	}
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	logFn("Querying latest release from GitHub: %s...\n", repo)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("query GitHub releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %s (check repo name or token)", resp.Status)
	}

	var rel struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", fmt.Errorf("decode release JSON: %w", err)
	}

	osKeyword := strings.ToLower(runtime.GOOS)
	archKeyword := runtime.GOARCH
	if archKeyword == "amd64" {
		archKeyword = "x86_64"
	}

	var matchedAsset *struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
	}

	for i := range rel.Assets {
		name := strings.ToLower(rel.Assets[i].Name)
		if strings.Contains(name, osKeyword) && (strings.Contains(name, archKeyword) || strings.Contains(name, runtime.GOARCH)) {
			matchedAsset = &rel.Assets[i]
			break
		}
	}

	if matchedAsset == nil && len(rel.Assets) > 0 {
		for i := range rel.Assets {
			if strings.Contains(strings.ToLower(rel.Assets[i].Name), osKeyword) {
				matchedAsset = &rel.Assets[i]
				break
			}
		}
	}

	if matchedAsset == nil {
		return "", fmt.Errorf("no matching release asset found for %s/%s in release %s", runtime.GOOS, runtime.GOARCH, rel.TagName)
	}

	logFn("Downloading %s (%s)...\n", matchedAsset.Name, rel.TagName)
	dlReq, err := http.NewRequestWithContext(ctx, "GET", matchedAsset.URL, nil)
	if err != nil {
		return "", err
	}
	dlReq.Header.Set("Accept", "application/octet-stream")
	if token != "" {
		dlReq.Header.Set("Authorization", "Bearer "+token)
	}

	dlResp, err := http.DefaultClient.Do(dlReq)
	if err != nil {
		return "", fmt.Errorf("download asset: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", dlResp.Status)
	}

	dlBytes, err := io.ReadAll(dlResp.Body)
	if err != nil {
		return "", fmt.Errorf("read asset body: %w", err)
	}

	installedName, err := p.InstallFromArchive(dlBytes, matchedAsset.Name, destDir)
	if err != nil {
		return "", fmt.Errorf("extract asset: %w", err)
	}

	logFn("✅ Plugin [%s] installed successfully into %s!\n", installedName, destDir)
	return installedName, nil
}

func (p *Plugin) InstallFromArchive(data []byte, filename, destDir string) (string, error) {
	if destDir == "" {
		destDir = p.DefaultPluginDir()
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return unpackTarGz(data, destDir)
	} else if strings.HasSuffix(lower, ".zip") {
		return unpackZip(data, destDir)
	} else if isExecutableFile(filename) {
		pluginName := parsePluginName(filename)
		subDir := filepath.Join(destDir, "ncmm-"+pluginName)
		_ = os.MkdirAll(subDir, 0755)
		targetFile := filepath.Join(subDir, filename)
		if err := os.WriteFile(targetFile, data, 0755); err != nil {
			return "", err
		}
		return pluginName, nil
	}
	return "", fmt.Errorf("unsupported file format: %s (expected .zip, .tar.gz, .tgz, or executable)", filename)
}

func unpackTarGz(data []byte, destDir string) (string, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	type fileEntry struct {
		name string
		data []byte
		mode os.FileMode
	}
	var entries []fileEntry
	var exeName string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Typeflag == tar.TypeReg {
			base := filepath.Base(hdr.Name)
			content, err := io.ReadAll(tr)
			if err != nil {
				return "", err
			}
			mode := os.FileMode(0644)
			if isExecutableFile(base) {
				mode = 0755
				if exeName == "" {
					exeName = base
				}
			}
			entries = append(entries, fileEntry{name: base, data: content, mode: mode})
		}
	}
	if exeName == "" {
		return "", fmt.Errorf("no executable found in archive")
	}

	pluginName := parsePluginName(exeName)
	pluginDir := filepath.Join(destDir, "ncmm-"+pluginName)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return "", err
	}

	for _, entry := range entries {
		targetPath := filepath.Join(pluginDir, entry.name)
		if err := os.WriteFile(targetPath, entry.data, entry.mode); err != nil {
			return "", err
		}
	}
	return pluginName, nil
}

func unpackZip(data []byte, destDir string) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}

	type fileEntry struct {
		name string
		data []byte
		mode os.FileMode
	}
	var entries []fileEntry
	var exeName string

	for _, file := range zr.File {
		if file.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(file.Name)
		rc, err := file.Open()
		if err != nil {
			return "", err
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return "", err
		}
		mode := os.FileMode(0644)
		if isExecutableFile(base) {
			mode = 0755
			if exeName == "" {
				exeName = base
			}
		}
		entries = append(entries, fileEntry{name: base, data: content, mode: mode})
	}
	if exeName == "" {
		return "", fmt.Errorf("no executable found in archive")
	}

	pluginName := parsePluginName(exeName)
	pluginDir := filepath.Join(destDir, "ncmm-"+pluginName)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return "", err
	}

	for _, entry := range entries {
		targetPath := filepath.Join(pluginDir, entry.name)
		if err := os.WriteFile(targetPath, entry.data, entry.mode); err != nil {
			return "", err
		}
	}
	return pluginName, nil
}
