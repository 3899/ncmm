// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package webui

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/3899/ncmm/internal/playstats"
	"github.com/3899/ncmm/pkg/database"
)

func (s *Server) handlePlayStatsGet(w http.ResponseWriter, r *http.Request) {
	document, err := s.config.get()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	databaseConfig, accounts, err := playStatsRuntimeConfig(document, s.opts.Home)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(accounts) == 0 {
		writeJSON(w, http.StatusOK, []playstats.AccountSeries{})
		return
	}

	db, err := database.NewWithOptions(databaseConfig, 1, 0, true)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "播放统计数据库正被任务占用，请稍后重试")
		return
	}
	defer db.Close(r.Context())

	dates := playstats.RecentCompleteDates(time.Now(), 7)
	series := make([]playstats.AccountSeries, 0, len(accounts))
	for _, account := range accounts {
		canonical := playstats.CanonicalAccount(account, s.opts.Home)
		points, loadErr := playstats.LoadSeries(r.Context(), db, canonical, dates)
		if loadErr != nil {
			writeError(w, http.StatusInternalServerError, loadErr.Error())
			return
		}
		series = append(series, playstats.AccountSeries{Account: account, Points: points})
	}
	writeJSON(w, http.StatusOK, series)
}

func playStatsRuntimeConfig(document configDocument, home string) (*database.Config, []string, error) {
	root, ok := document.Data.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("配置文件根节点格式无效")
	}
	databaseNode, ok := root["database"].(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("未配置本地数据库")
	}
	driver, _ := databaseNode["driver"].(string)
	path, _ := databaseNode["path"].(string)
	path = resolvePlayStatsPath(path, home)
	if path == "" {
		return nil, nil, fmt.Errorf("未配置本地数据库路径")
	}
	return &database.Config{Driver: strings.TrimSpace(driver), Path: path}, configuredPlayStatAccounts(root), nil
}

func configuredPlayStatAccounts(root map[string]any) []string {
	accountsNode, ok := root["accounts"].(map[string]any)
	if !ok {
		return nil
	}
	var accounts []string
	main, _ := accountsNode["main"].(string)
	if strings.TrimSpace(main) == "" {
		main, _ = accountsNode["primary"].(string)
	}
	if main = strings.TrimSpace(main); main != "" {
		accounts = append(accounts, main)
	}
	switch secondary := accountsNode["secondary"].(type) {
	case []any:
		for _, item := range secondary {
			if path, ok := item.(string); ok && strings.TrimSpace(path) != "" {
				accounts = append(accounts, strings.TrimSpace(path))
			}
		}
	case []string:
		for _, path := range secondary {
			if strings.TrimSpace(path) != "" {
				accounts = append(accounts, strings.TrimSpace(path))
			}
		}
	}
	return accounts
}

func resolvePlayStatsPath(path, home string) string {
	path = strings.TrimSpace(strings.ReplaceAll(path, "${HOME}", home))
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(home, path)
	}
	return filepath.Clean(path)
}
