package main

import (
	"encoding/json"
	"net/http"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/prefs"
)

// cliDetector 探测 PATH 中已安装的 cc-select CLI 状态（真实实现见 cliinstall.go，单测用 fake）。
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

// installFn 是安装器注入缝（真身 installEmbeddedCLI；端点契约测试注入 fake）。
type installFn func(installOptions) (installResult, *installConflict, error)

// desktopAPIOptions 聚合桌面专属端点的依赖注入。
type desktopAPIOptions struct {
	AppVersion         string
	EmbeddedCliVersion string
	Cli                cliDetector
	TargetDirs         func() []string
	ExtractCli         func(dest string) error
	// Install 可选；nil 时用 installEmbeddedCLI + TargetDirs/ExtractCli 组装。
	Install installFn
	// 偏好存取（settings 端点）；nil 时用 prefs.Load/Save。
	LoadPrefs func() (*prefs.Prefs, error)
	SavePrefs func(*prefs.Prefs) error
	// SyncServices 在 PUT settings 落盘前同步 OS 层服务（托盘驻留/自启）；
	// 返回错误则整体回滚。nil 视为无服务可同步（仅落盘）。
	SyncServices func(prefs.DesktopPrefs) error
	// ListProviders 供托盘命令端点读取 provider 清单；nil 时用 config.Load。
	ListProviders func() ([]config.Provider, error)
}

// desktopAPI 伺服 /api/v1/desktop/ 命名空间（info/cli 安装起步，settings/tray 随 US3–US5 递增）。
type desktopAPI struct {
	desktopAPIOptions
}

// newDesktopAPI 组装桌面专属端点。版本由调用方注入（构建期 ldflags 统一落
// internal/version.Version，App 与内置 CLI 同源）。
func newDesktopAPI(opts desktopAPIOptions) http.Handler {
	a := &desktopAPI{opts}
	if a.Install == nil {
		a.Install = func(o installOptions) (installResult, *installConflict, error) {
			o.TargetDirs = a.TargetDirs()
			o.ExtractEmbedded = a.ExtractCli
			return installEmbeddedCLI(o)
		}
	}
	if a.LoadPrefs == nil {
		a.LoadPrefs = prefs.Load
	}
	if a.SavePrefs == nil {
		a.SavePrefs = prefs.Save
	}
	if a.ListProviders == nil {
		a.ListProviders = sortedProviders
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/desktop/info", a.handleInfo)
	mux.HandleFunc("/api/v1/desktop/cli/install", a.handleCliInstall)
	mux.HandleFunc("/api/v1/desktop/settings", a.handleSettings)
	mux.HandleFunc("/api/v1/desktop/tray/commands", a.handleTrayCommands)
	return mux
}

func (a *desktopAPI) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	dto := desktopInfoDTO{
		Mode:               "desktop",
		AppVersion:         a.AppVersion,
		EmbeddedCliVersion: a.EmbeddedCliVersion,
	}
	if path, ver, found := a.Cli.Detect(); found {
		dto.Cli = cliInfoDTO{
			InstalledPath:    &path,
			InstalledVersion: &ver,
			UpdateAvailable:  ver != a.EmbeddedCliVersion,
		}
	}
	writeJSONBody(w, http.StatusOK, dto)
}

// handleCliInstall POST /api/v1/desktop/cli/install（契约 §3.3）。
func (a *desktopAPI) handleCliInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		ConfirmOverwrite bool `json:"confirmOverwrite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	res, conflict, err := a.Install(installOptions{
		EmbeddedVersion:  a.EmbeddedCliVersion,
		ConfirmOverwrite: body.ConfirmOverwrite,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if conflict != nil {
		writeJSONBody(w, http.StatusConflict, conflict)
		return
	}
	writeJSONBody(w, http.StatusOK, res)
}
