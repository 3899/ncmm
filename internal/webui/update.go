package webui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type updateState struct {
	LastCheckTime  time.Time `json:"last_check_time"`
	CurrentVersion string    `json:"current_version"`
	LatestVersion  string    `json:"latest_version"`
	ReleaseNotes   string    `json:"release_notes"`
	UpdateStatus   string    `json:"update_status"`
	UpdatedVersion string    `json:"updated_version"`
	LastUpdateTime time.Time `json:"last_update_time"`
	LastError      string    `json:"last_error"`
	OS             string    `json:"os"`
	Arch           string    `json:"arch"`
}

type updateView struct {
	updateState
	Available       bool   `json:"available"`
	CanApply        bool   `json:"canApply"`
	Docker          bool   `json:"docker"`
	RestartRequired bool   `json:"restartRequired"`
	Message         string `json:"message,omitempty"`
}

func (s *Server) handleUpdateGet(w http.ResponseWriter, _ *http.Request) {
	state := updateState{CurrentVersion: s.opts.Version, LatestVersion: s.opts.Version, UpdateStatus: "idle"}
	data, err := os.ReadFile(filepath.Join(s.opts.Home, "ncmm-update.json"))
	if err == nil {
		_ = json.Unmarshal(data, &state)
	} else if !os.IsNotExist(err) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.makeUpdateView(state, false))
}

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	state, err := s.runUpdateCommand(r.Context(), false)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.makeUpdateView(state, false))
}

func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("NCMM_DOCKER_OFFICIAL") == "1" {
		writeError(w, http.StatusConflict, "官方 Docker 容器不能替换镜像内二进制，请执行 docker compose pull && docker compose up -d")
		return
	}
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	state, err := s.runUpdateCommand(r.Context(), true)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.makeUpdateView(state, true))
}

func (s *Server) runUpdateCommand(parent context.Context, apply bool) (updateState, error) {
	timeout := 45 * time.Second
	if apply {
		timeout = 4 * time.Minute
	}
	args := []string{"--config", s.opts.ConfigPath, "--home", s.opts.Home, "update", "--json"}
	if apply {
		args = append(args, "--apply")
	}
	output, err := s.processes.RunOutput(parent, processSpec{
		Kind: "update", Command: s.opts.Executable, Args: args, Dir: s.opts.Home,
		Env: append(os.Environ(), "NCMM_WEB_CHILD=1"), Timeout: timeout,
	}, defaultProcessOutputLimit)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(parent.Err(), context.DeadlineExceeded) {
		return updateState{}, fmt.Errorf("更新操作超时: %w", context.DeadlineExceeded)
	}
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return updateState{}, fmt.Errorf("更新命令失败: %s", message)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		var state updateState
		if json.Unmarshal([]byte(strings.TrimSpace(lines[i])), &state) == nil && state.CurrentVersion != "" {
			return state, nil
		}
	}
	return updateState{}, fmt.Errorf("更新命令未返回有效状态")
}

func (s *Server) makeUpdateView(state updateState, restartRequired bool) updateView {
	if strings.TrimSpace(s.opts.Version) != "" {
		state.CurrentVersion = strings.TrimSpace(s.opts.Version)
	} else if strings.TrimSpace(state.CurrentVersion) == "" {
		state.CurrentVersion = s.opts.Version
	}
	if strings.TrimSpace(state.LatestVersion) == "" {
		state.LatestVersion = state.CurrentVersion
	}
	if strings.TrimSpace(state.OS) == "" {
		state.OS = runtime.GOOS
	}
	if strings.TrimSpace(state.Arch) == "" {
		state.Arch = runtime.GOARCH
	}
	docker := os.Getenv("NCMM_DOCKER_OFFICIAL") == "1"
	available := compareUpdateVersions(state.CurrentVersion, state.LatestVersion) < 0
	installed := available && state.UpdatedVersion != "" && compareUpdateVersions(state.UpdatedVersion, state.LatestVersion) >= 0
	restartRequired = restartRequired || installed
	view := updateView{
		updateState: state, Available: available, CanApply: available && !docker && !installed,
		Docker: docker, RestartRequired: restartRequired,
	}
	if docker && available {
		view.Message = "检测到新版本。Docker 部署请拉取新镜像并重建容器。"
	} else if restartRequired {
		view.Message = "更新文件已安装，请重启 ncmm 后生效。"
	} else if !available && !state.LastCheckTime.IsZero() {
		view.Message = "当前已是最新版本。"
	}
	return view
}

func compareUpdateVersions(left, right string) int {
	parse := func(version string) []int {
		version = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(version), "v"), "V")
		parts := strings.Split(version, ".")
		values := make([]int, 0, len(parts))
		for _, part := range parts {
			digits := strings.TrimLeftFunc(part, func(r rune) bool { return r < '0' || r > '9' })
			end := strings.IndexFunc(digits, func(r rune) bool { return r < '0' || r > '9' })
			if end >= 0 {
				digits = digits[:end]
			}
			value, _ := strconv.Atoi(digits)
			values = append(values, value)
		}
		return values
	}
	a, b := parse(left), parse(right)
	for i := 0; i < len(a) || i < len(b); i++ {
		var av, bv int
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}
