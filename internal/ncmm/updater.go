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
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/3899/ncmm/config"
	"github.com/3899/ncmm/pkg/log"
	"github.com/3899/ncmm/pkg/utils"
)

var (
	updaterMu   sync.Mutex
	stateFileMu sync.Mutex
)

// UpdateState 定义跟目录 ncmm-update.json 中保存的状态元数据
type UpdateState struct {
	// 基础检测信息
	LastCheckTime  time.Time `json:"last_check_time"` // 上次网络 API 检测时间
	CurrentVersion string    `json:"current_version"` // 检查时运行版本
	LatestVersion  string    `json:"latest_version"`  // 远程最新版本号 (如 1.1.14)
	ReleaseNotes   string    `json:"release_notes"`   // 最新版本的更新说明/日志摘要

	// 自动热更新状态
	UpdateStatus   string    `json:"update_status"`    // 状态: "idle" | "updating" | "success" | "failed"
	UpdatedVersion string    `json:"updated_version"`  // 已成功替换完成的目标版本号
	LastUpdateTime time.Time `json:"last_update_time"` // 上次热替换成功完成时间
	LastError      string    `json:"last_error"`       // 上次失败错误日志

	// 系统架构及目标资产信息
	OS            string `json:"os"`           // 操作系统 (如 Windows / Linux / Darwin)
	Arch          string `json:"arch"`         // 系统架构 (如 x86_64 / arm64)
	AssetFilename string `json:"target_asset"` // 匹配的压缩包文件名 (如 ncmm_Linux_x86_64.tar.gz)
	DownloadURL   string `json:"download_url"` // 资产原始下载链接
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Body    string        `json:"body"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// getHomePath 获取工作目录家目录
func (c *Root) getHomePath() string {
	if c.Opts.Home != "" {
		return filepath.Clean(c.Opts.Home)
	}
	return filepath.Clean(config.HomeDir)
}

// getUpdateStatePath 获取 ncmm-update.json 绝对路径
func (c *Root) getUpdateStatePath() string {
	return filepath.Join(c.getHomePath(), "ncmm-update.json")
}

// initDefaultUpdateState 基于当前二进制版本和操作系统架构初始化默认状态
func (c *Root) initDefaultUpdateState() UpdateState {
	currentVer := c.AppVersion
	if currentVer == "" {
		currentVer = "0.0.0"
	}
	osPart, archPart, ext := getPlatformInfo()
	targetName := fmt.Sprintf("ncmm_%s_%s%s", osPart, archPart, ext)
	downloadURL := fmt.Sprintf("https://github.com/3899/ncmm/releases/download/v%s/%s", strings.TrimPrefix(currentVer, "v"), targetName)

	return UpdateState{
		LastCheckTime:  time.Unix(0, 0), // 初始设为 Epoch 时间，确保首次允许触发网络检测
		CurrentVersion: currentVer,
		LatestVersion:  currentVer,
		UpdateStatus:   "idle",
		OS:             osPart,
		Arch:           archPart,
		AssetFilename:  targetName,
		DownloadURL:    downloadURL,
	}
}

// saveUpdateState 并发安全地保存 ncmm-update.json 状态数据
func (c *Root) saveUpdateState(statePath string, state *UpdateState) {
	stateFileMu.Lock()
	defer stateFileMu.Unlock()
	if data, err := json.MarshalIndent(state, "", "  "); err == nil {
		_ = os.MkdirAll(filepath.Dir(statePath), 0755)
		if err := os.WriteFile(statePath, data, 0644); err != nil {
			log.Warn("[updater] 写入 ncmm-update.json 状态文件失败: %v", err)
		}
	}
}

// CleanOldExecutable 清理上一次更新留下的 .old 临时备份文件
func (c *Root) CleanOldExecutable() {
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	oldPath := execPath + ".old"
	if utils.FileExists(oldPath) {
		_ = os.Remove(oldPath)
	}
}

// CheckForUpdatesPreRun 执行任务前阶段的版本检测与同步热替换
func (c *Root) CheckForUpdatesPreRun() {
	c.CleanOldExecutable()

	// 获取配置中的 updater 控制参数 (支持配置文件与 NCMM_UPDATER_CHECK / NCMM_UPDATER_AUTO_UPDATE 环境变量)
	checkEnabled := true
	autoUpdateEnabled := true
	if c.Cfg != nil && c.Cfg.Updater != nil {
		if c.Cfg.Updater.Check != nil {
			checkEnabled = *c.Cfg.Updater.Check
		}
		if c.Cfg.Updater.AutoUpdate != nil {
			autoUpdateEnabled = *c.Cfg.Updater.AutoUpdate
		}
	}

	// 显式支持环境变量覆写判断 (如 NCMM_UPDATER_CHECK=false 或 0)
	if isEnvFalse("NCMM_UPDATER_CHECK") {
		checkEnabled = false
	}
	if isEnvFalse("NCMM_UPDATER_AUTO_UPDATE") {
		autoUpdateEnabled = false
	}

	if !checkEnabled {
		log.Debug("[updater] 版本自动检测与升级功能已关闭 (updater.check=false)")
		return
	}

	statePath := c.getUpdateStatePath()
	var state UpdateState

	// 仅当 ncmm-update.json 不存在时（新系统首次初始化 / 旧版本升级过渡）
	if !utils.FileExists(statePath) {
		// 顺手擦除历史残留文件 update_cache.json
		_ = os.Remove(filepath.Join(c.getHomePath(), "update_cache.json"))
		state = c.initDefaultUpdateState()
		c.saveUpdateState(statePath, &state)
	} else {
		// 只要 ncmm-update.json 已存在，直接读取
		if data, err := os.ReadFile(statePath); err == nil {
			_ = json.Unmarshal(data, &state)
		}
	}

	currentVer := c.AppVersion
	if currentVer == "" {
		currentVer = "0.0.0"
	}

	isDockerOfficial := os.Getenv("NCMM_DOCKER_OFFICIAL") == "1"
	needAutoUpdate := autoUpdateEnabled && !isDockerOfficial

	// 1. 命中已有的状态记录：如果记录的最新版本大于当前版本
	if state.LatestVersion != "" && CompareVersions(currentVer, state.LatestVersion) < 0 {
		log.Info("[updater] 发现可用新版本: %s (当前版本: %s)", state.LatestVersion, currentVer)
		if needAutoUpdate && state.UpdatedVersion != state.LatestVersion {
			_ = c.performSelfUpdateFromState(&state, statePath)
			return
		}
	}

	// 2. 频率控制：如果距离上次 API 网络检测不足 24 小时，不重复发起网络请求
	if time.Since(state.LastCheckTime) < 24*time.Hour {
		return
	}

	// 3. 发起同步网络 API 检测并处理热替换
	_, _ = c.checkNewVersionSync(statePath, currentVer, needAutoUpdate)
}

func (c *Root) checkNewVersionSync(statePath string, currentVer string, autoUpdateEnabled bool) (UpdateState, error) {
	state := c.initDefaultUpdateState()
	if data, err := os.ReadFile(statePath); err == nil {
		_ = json.Unmarshal(data, &state)
	}
	proxyMirrors := []string{"https://gh-proxy.com/", "https://ghproxy.net/", "https://githubproxy.cc/"}
	if c.Cfg != nil && c.Cfg.Updater != nil && len(c.Cfg.Updater.ProxyMirrors) > 0 {
		proxyMirrors = c.Cfg.Updater.ProxyMirrors
	}

	apiURL := "https://api.github.com/repos/3899/ncmm/releases/latest"
	urlsToTry := []string{apiURL}
	for _, proxy := range proxyMirrors {
		if proxy != "" {
			urlsToTry = append(urlsToTry, proxy+apiURL)
		}
	}

	var resp *http.Response
	var responseCancel context.CancelFunc
	var lastErr error
	var succeeded bool

	for _, targetURL := range urlsToTry {
		log.Debug("[updater] 正在获取最新版本信息: %s", targetURL)

		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "ncmm-updater/"+currentVer)

		resp, lastErr = http.DefaultClient.Do(req)
		if lastErr == nil {
			if resp.StatusCode == http.StatusOK {
				responseCancel = cancel
				succeeded = true
				break
			}
			lastErr = fmt.Errorf("HTTP status error: %d", resp.StatusCode)
		}
		cancel()
		if resp != nil {
			resp.Body.Close()
		}
	}

	if !succeeded {
		log.Debug("[updater] 获取最新版本信息失败: %v", lastErr)
		return state, fmt.Errorf("获取最新版本信息失败: %w", lastErr)
	}
	defer responseCancel()
	defer resp.Body.Close()

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		log.Warn("[updater] 解析版本信息失败: %v", err)
		return state, fmt.Errorf("解析版本信息失败: %w", err)
	}

	tag := strings.TrimSpace(rel.TagName)
	if tag == "" {
		return state, fmt.Errorf("最新版本响应缺少 tag_name")
	}

	osPart, archPart, ext := getPlatformInfo()
	targetName := fmt.Sprintf("ncmm_%s_%s%s", osPart, archPart, ext)
	var downloadURL string
	for _, asset := range rel.Assets {
		if strings.EqualFold(asset.Name, targetName) {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		downloadURL = fmt.Sprintf("https://github.com/3899/ncmm/releases/download/%s/%s", tag, targetName)
	}

	state.LastCheckTime = time.Now()
	state.CurrentVersion = currentVer
	state.LatestVersion = tag
	state.ReleaseNotes = rel.Body
	state.OS = osPart
	state.Arch = archPart
	state.AssetFilename = targetName
	state.DownloadURL = downloadURL

	c.saveUpdateState(statePath, &state)

	if CompareVersions(currentVer, tag) >= 0 {
		return state, nil
	}

	log.Info("[updater] 发现可用新版本: %s (当前版本: %s)", tag, currentVer)

	if autoUpdateEnabled && state.UpdatedVersion != tag {
		_ = c.performSelfUpdateFromState(&state, statePath)
	}
	return state, nil
}

// performSelfUpdateFromState 从状态结构中提取配置并执行全自动热替换
func (c *Root) performSelfUpdateFromState(state *UpdateState, statePath string) error {
	state.UpdateStatus = "updating"
	c.saveUpdateState(statePath, state)

	log.Info("[updater] 正在自动升级至 %s ...", state.LatestVersion)
	err := c.performSelfUpdateWithInfo(state.LatestVersion, state.AssetFilename, state.DownloadURL)
	if err != nil {
		state.UpdateStatus = "failed"
		state.LastError = err.Error()
		c.saveUpdateState(statePath, state)
		log.Warn("[updater] 自动升级失败: %s", err)
		return err
	} else {
		state.UpdateStatus = "success"
		state.UpdatedVersion = state.LatestVersion
		state.LastUpdateTime = time.Now()
		state.LastError = ""
		c.saveUpdateState(statePath, state)
		log.Info("[updater] ncmm 已成功升级至 %s 版本", state.LatestVersion)
	}
	return nil
}

// CheckForUpdatesNow bypasses the daily throttle for an explicit user request.
func (c *Root) CheckForUpdatesNow() (UpdateState, error) {
	updaterMu.Lock()
	defer updaterMu.Unlock()
	currentVer := strings.TrimSpace(c.AppVersion)
	if currentVer == "" {
		currentVer = "0.0.0"
	}
	return c.checkNewVersionSync(c.getUpdateStatePath(), currentVer, false)
}

// ApplyAvailableUpdate installs the release recorded by the latest manual check.
func (c *Root) ApplyAvailableUpdate() (UpdateState, error) {
	updaterMu.Lock()
	defer updaterMu.Unlock()
	statePath := c.getUpdateStatePath()
	state := c.initDefaultUpdateState()
	if data, err := os.ReadFile(statePath); err == nil {
		_ = json.Unmarshal(data, &state)
	}
	currentVer := strings.TrimSpace(c.AppVersion)
	if currentVer == "" {
		currentVer = "0.0.0"
	}
	if os.Getenv("NCMM_DOCKER_OFFICIAL") == "1" {
		return state, fmt.Errorf("官方 Docker 容器不支持替换镜像内二进制，请拉取新镜像后重建容器")
	}
	if state.LatestVersion == "" || CompareVersions(currentVer, state.LatestVersion) >= 0 {
		return state, fmt.Errorf("当前没有可应用的新版本，请先检查更新")
	}
	if err := c.performSelfUpdateFromState(&state, statePath); err != nil {
		return state, err
	}
	return state, nil
}

// performSelfUpdateWithInfo 执行真正的升级下载与文件覆盖
func (c *Root) performSelfUpdateWithInfo(tag string, assetName string, rawDownloadURL string) error {
	osPart, archPart, ext := getPlatformInfo()
	if assetName == "" {
		assetName = fmt.Sprintf("ncmm_%s_%s%s", osPart, archPart, ext)
	}
	if rawDownloadURL == "" {
		rawDownloadURL = fmt.Sprintf("https://github.com/3899/ncmm/releases/download/%s/%s", tag, assetName)
	}

	proxyMirrors := []string{"https://gh-proxy.com/", "https://ghproxy.net/", "https://githubproxy.cc/"}
	if c.Cfg != nil && c.Cfg.Updater != nil && len(c.Cfg.Updater.ProxyMirrors) > 0 {
		proxyMirrors = c.Cfg.Updater.ProxyMirrors
	}

	urlsToTry := []string{rawDownloadURL}
	for _, proxy := range proxyMirrors {
		if proxy != "" {
			urlsToTry = append(urlsToTry, proxy+rawDownloadURL)
		}
	}

	var resp *http.Response
	var responseCancel context.CancelFunc
	var lastErr error
	var succeeded bool

	for _, targetURL := range urlsToTry {
		log.Debug("[updater] 正在下载更新包: %s", targetURL)

		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "ncmm-updater")

		resp, lastErr = http.DefaultClient.Do(req)
		if lastErr == nil {
			if resp.StatusCode == http.StatusOK {
				responseCancel = cancel
				succeeded = true
				break
			}
			lastErr = fmt.Errorf("HTTP status error: %d", resp.StatusCode)
		}
		cancel()
		if resp != nil {
			resp.Body.Close()
		}
		log.Warn("[updater] 从 %s 下载更新包失败，正在重试备用源...", targetURL)
	}

	if !succeeded {
		return fmt.Errorf("下载更新包失败，已尝试所有下载方式。最后一次错误: %w", lastErr)
	}
	defer responseCancel()
	defer resp.Body.Close()

	archiveBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var binaryBytes []byte
	binaryName := "ncmm"
	if runtime.GOOS == "windows" {
		binaryName = "ncmm.exe"
	}

	if ext == ".zip" {
		zipReader, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
		if err != nil {
			return fmt.Errorf("zip.NewReader: %w", err)
		}
		for _, file := range zipReader.File {
			if strings.EqualFold(filepath.Base(file.Name), binaryName) {
				rc, err := file.Open()
				if err != nil {
					return err
				}
				binaryBytes, err = io.ReadAll(rc)
				rc.Close()
				if err != nil {
					return err
				}
				break
			}
		}
	} else {
		// tar.gz
		gzipReader, err := gzip.NewReader(bytes.NewReader(archiveBytes))
		if err != nil {
			return fmt.Errorf("gzip.NewReader: %w", err)
		}
		defer gzipReader.Close()

		tarReader := tar.NewReader(gzipReader)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if strings.EqualFold(filepath.Base(header.Name), binaryName) {
				binaryBytes, err = io.ReadAll(tarReader)
				if err != nil {
					return err
				}
				break
			}
		}
	}

	if len(binaryBytes) == 0 {
		return fmt.Errorf("在升级包中找不到可执行文件: %s", binaryName)
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	oldPath := execPath + ".old"
	if utils.FileExists(oldPath) {
		_ = os.Remove(oldPath)
	}
	if err := os.Rename(execPath, oldPath); err != nil {
		return fmt.Errorf("重命名原程序失败: %w", err)
	}

	if err := os.WriteFile(execPath, binaryBytes, 0755); err != nil {
		_ = os.Rename(oldPath, execPath)
		return fmt.Errorf("写入新二进制文件失败: %w", err)
	}

	if c.CfgPath != "" && c.CfgPath != "default" {
		if err := config.AutoUpgradeConfig(c.CfgPath); err != nil {
			log.Warn("[updater] 配置文件自动升级合并失败: %s", err)
		}
	}

	return nil
}

// ShowUpdateNotificationPostRun 在 PersistentPostRun 阶段展示更新提醒（非强制自动更新模式下）
func (c *Root) ShowUpdateNotificationPostRun() {
	// PostRun 阶段仅需在检测到有更新但未开启 auto_update 时提醒用户
}

// CompareVersions 比对版本号。v1 > v2 返回 1，v1 < v2 返回 -1，相等返回 0
func CompareVersions(v1, v2 string) int {
	p1 := parseVersion(v1)
	p2 := parseVersion(v2)

	for i := 0; i < len(p1) || i < len(p2); i++ {
		var val1, val2 int
		if i < len(p1) {
			val1 = p1[i]
		}
		if i < len(p2) {
			val2 = p2[i]
		}
		if val1 < val2 {
			return -1
		} else if val1 > val2 {
			return 1
		}
	}
	return 0
}

func parseVersion(v string) []int {
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	parts := strings.Split(v, ".")
	res := make([]int, 0, len(parts))
	for _, p := range parts {
		var digits string
		for _, r := range p {
			if r >= '0' && r <= '9' {
				digits += string(r)
			} else {
				break
			}
		}
		if digits != "" {
			val, _ := strconv.Atoi(digits)
			res = append(res, val)
		} else {
			res = append(res, 0)
		}
	}
	return res
}

func getPlatformInfo() (string, string, string) {
	osPart := runtime.GOOS
	switch runtime.GOOS {
	case "windows":
		osPart = "Windows"
	case "linux":
		osPart = "Linux"
	case "darwin":
		osPart = "Darwin"
	default:
		if len(osPart) > 0 {
			osPart = strings.ToUpper(osPart[:1]) + osPart[1:]
		}
	}

	archPart := runtime.GOARCH
	switch runtime.GOARCH {
	case "amd64":
		archPart = "x86_64"
	case "arm64":
		archPart = "arm64"
	case "arm":
		archPart = "armv6"
	}

	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}

	return osPart, archPart, ext
}

func isEnvFalse(key string) bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return val == "0" || val == "false" || val == "off" || val == "no"
}
