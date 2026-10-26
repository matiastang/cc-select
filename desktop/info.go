package main

import (
	"encoding/json"
	"net/http"
)

// cliDetector 探测 PATH 中已安装的 cc-select CLI 状态（US2/T015 提供真实实现，单测用 fake）。
type cliDetector interface {
	// Detect 返回已安装路径、版本与是否找到。found=false 时前两个返回值无意义。
	Detect() (path, version string, found bool)
}

// cliInfoDTO 是 info 响应的 cli 子结构（data-model §1.2/§2.1）。
type cliInfoDTO struct {
	InstalledPath    *string `json:"installedPath"`
	InstalledVersion *string `json:"installedVersion"`
	UpdateAvailable  bool    `json:"updateAvailable"`
}

// desktopInfoDTO 是 GET /api/v1/desktop/info 的响应（data-model §2.1）。
// 前端以该端点探测桌面能力：web 模式（ccs gui）下不存在 → 404 → 隐藏 Desktop 设置区。
type desktopInfoDTO struct {
	Mode               string     `json:"mode"`
	AppVersion         string     `json:"appVersion"`
	EmbeddedCliVersion string     `json:"embeddedCliVersion"`
	Cli                cliInfoDTO `json:"cli"`
}

// desktopAPI 伺服 /api/v1/desktop/ 命名空间（info 起步，settings/cli/tray 随 US2–US5 递增）。
type desktopAPI struct {
	appVersion         string
	embeddedCliVersion string
	cli                cliDetector
}

// newDesktopAPI 组装桌面专属端点。版本由调用方注入（构建期 ldflags 统一落
// internal/version.Version，App 与内置 CLI 同源）。
func newDesktopAPI(appVersion, embeddedCliVersion string, cli cliDetector) http.Handler {
	a := &desktopAPI{appVersion: appVersion, embeddedCliVersion: embeddedCliVersion, cli: cli}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/desktop/info", a.handleInfo)
	return mux
}

func (a *desktopAPI) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	dto := desktopInfoDTO{
		Mode:               "desktop",
		AppVersion:         a.appVersion,
		EmbeddedCliVersion: a.embeddedCliVersion,
	}
	if path, ver, found := a.cli.Detect(); found {
		dto.Cli = cliInfoDTO{
			InstalledPath:    &path,
			InstalledVersion: &ver,
			UpdateAvailable:  ver != a.embeddedCliVersion,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto)
}
