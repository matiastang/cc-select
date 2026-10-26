// Command desktop 是 cc-select 的 Wails v3 桌面客户端入口。
//
// 与 CLI（仓库根 main.go）物理隔离：本包只做"壳"——窗口、资产服务、托盘、
// 更新与系统集成；业务逻辑全部复用 internal/* 包（同一份 ~/.cc-select 数据）。
// 设计与契约见 specs/003-desktop-client/（plan.md / contracts/）。
package main

import (
	"log"
	"net/http"

	"github.com/cc-select/cc-select/internal/version"
	"github.com/cc-select/cc-select/internal/web"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func main() {
	appVersion := version.Version

	// 内容层：既有 Web GUI 的完整 handler（API + 静态资产，零改动复用）。
	// 桌面语义叠加：update 桥（US3 接 Wails Updater）+ desktop 命名空间端点。
	content := web.Handler()
	update := newStubUpdateBridge(appVersion)
	desktopAPI := newDesktopAPI(appVersion, appVersion, &notInstalledDetector{})

	app := application.New(application.Options{
		Name:        "CC Select",
		Description: "cc-select 桌面客户端",
		Assets: application.AssetOptions{
			Handler: newBridge(content, update, desktopAPI),
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "CC Select",
		Width:     1120,
		Height:    760,
		MinWidth:  880,
		MinHeight: 600,
		URL:       "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// newStubUpdateBridge 是 US1 阶段的更新桥占位：check 返回与 web 端点同构的
// 「无更新」响应（前端卡片正常渲染），POST 一律 501（桌面 App 绝不能触发
// internal/updater 的 CLI 自替换路径——它会替换桌面二进制本身）。
// US3（T022）以 Wails Updater 真身替换。
func newStubUpdateBridge(appVersion string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/update/check", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeJSONBody(w, http.StatusOK, map[string]any{
			"currentVersion": appVersion,
			"latestVersion":  appVersion,
			"hasUpdate":      false,
			"devBuild":       appVersion == "dev",
		})
	})
	mux.HandleFunc("/api/v1/update", func(w http.ResponseWriter, r *http.Request) {
		writeJSONError(w, http.StatusNotImplemented, "desktop update arrives in US3")
	})
	return mux
}

// notInstalledDetector 是 CLI 探测占位（US2/T015 提供 PATH 真实探测）。
type notInstalledDetector struct{}

func (notInstalledDetector) Detect() (string, string, bool) { return "", "", false }
