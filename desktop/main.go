// Command desktop 是 cc-select 的 Wails v3 桌面客户端入口。
//
// 与 CLI（仓库根 main.go）物理隔离：本包只做"壳"——窗口、资产服务、托盘、
// 更新与系统集成；业务逻辑全部复用 internal/* 包（同一份 ~/.cc-select 数据）。
// 设计与契约见 specs/003-desktop-client/（plan.md / contracts/）。
package main

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/cc-select/cc-select/internal/version"
	"github.com/cc-select/cc-select/internal/web"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

// 更新通道（contracts/release-assets.md）：与 CLI 同 Release，桌面资产以
// cc-select-desktop 前缀区分（AssetMatcher 据此排除同名平台子串的 CLI 资产）。
const updateRepository = "matiastang/cc-select"

func main() {
	appVersion := version.Version

	// 后端文案（托盘菜单/通知）语言与 CLI/Web 共享同一份偏好（FR-012）。
	if p, err := prefs.Load(); err == nil {
		i18n.SetLocale(i18n.ResolveLocale(p.Language))
	}

	// 内容层：既有 Web GUI 的完整 handler（API + 静态资产，零改动复用）。
	// 桌面语义叠加：update 桥（Wails Updater）+ desktop 命名空间端点。
	notifier := notifications.New()
	updaterRef := &updaterHolder{}     // app.Updater 是 application.New 的产物，handler 构造先于其存在，惰性绑定
	autostartRef := &autostartHolder{} // AutostartManager 同理（US5）

	var window *application.WebviewWindow
	var tray *trayController

	app := application.New(application.Options{
		Name:        "CC Select",
		Description: "cc-select 桌面客户端",
		Services:    []application.Service{application.NewService(notifier)},
		Assets: application.AssetOptions{
			Handler: newBridge(web.Handler(), newUpdateBridge(appVersion, updaterRef), newDesktopAPI(desktopAPIOptions{
				AppVersion:         appVersion,
				EmbeddedCliVersion: appVersion, // App 与内置 CLI 同源构建（contracts/release-assets.md §5）
				Cli:                pathCliDetector{},
				TargetDirs:         defaultInstallDirs,
				ExtractCli:         extractEmbeddedCli,
				// US5：自启经 v3 AutostartManager 同步（失败 503 整体回滚）。
				// 托盘驻留无需 OS 注册（关窗钩子读 prefs 即时生效）。
				SyncServices: autostartRef.sync,
			})),
		},
		// FR-011：单实例——二次启动聚焦既有窗口，不产生第二进程/托盘。
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.matiastang.cc-select.desktop",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if window != nil {
					window.Restore()
					window.Focus()
				}
			},
		},
	})

	updaterRef.svc = app.Updater
	autostartRef.svc = app.Autostart
	initUpdater(app, appVersion)

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "CC Select",
		Width:     1120,
		Height:    760,
		MinWidth:  880,
		MinHeight: 600,
		URL:       "/",
	})

	// US4：托盘常驻 + 关窗驻留/退出语义（FR-008/FR-009）。
	tray = newTrayController(app, window, notifier)
	window.RegisterHook(events.Common.WindowClosing, tray.onWindowClosing)

	go startupUpdateCheck(app, notifier, appVersion)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// updaterHolder 让更新桥在 application.New 之前就能构造（svc 稍后绑定为 app.Updater）。
type updaterHolder struct {
	svc updaterService
}

func (h *updaterHolder) Check(ctx context.Context) (*updater.Release, error) {
	if h.svc == nil {
		return nil, errUpdaterNotReady
	}
	return h.svc.Check(ctx)
}
func (h *updaterHolder) DownloadAndInstall(ctx context.Context) error {
	if h.svc == nil {
		return errUpdaterNotReady
	}
	return h.svc.DownloadAndInstall(ctx)
}

// errUpdaterNotReady 覆盖"绑定前被调用"的理论窗口（与 autostartHolder 一致）。
var errUpdaterNotReady = errors.New("updater not ready")

// initUpdater 配置 Wails Updater：GitHub provider + 桌面资产匹配 + SHA256SUMS 摘要校验。
func initUpdater(app *application.App, appVersion string) {
	gh, err := github.New(github.Config{
		Repository:    updateRepository,
		AssetMatcher:  desktopAssetMatcher,
		ChecksumAsset: "SHA256SUMS",
	})
	if err != nil {
		log.Printf("updater: github provider: %v", err)
		return
	}
	// CurrentVersion 约定无 v 前缀（updater.Config 注释）；git describe 带 v，剥掉。
	// PublicKey 未配置 → digest-only（SHA256SUMS 流式校验）；minisign 全签名列为增强
	//（contracts/release-assets.md §4）。
	if err := app.Updater.Init(updater.Config{
		CurrentVersion: strings.TrimPrefix(appVersion, "v"),
		Providers:      []updater.Provider{gh},
		Window:         updater.WindowNone,
	}); err != nil {
		log.Printf("updater: init: %v", err)
	}
}

// desktopAssetMatcher 先按 cc-select-desktop 前缀过滤（排除含相同平台子串的
// CLI 资产），再交给默认平台/架构匹配。
func desktopAssetMatcher(req updater.CheckRequest, assets []github.ReleaseAsset) int {
	filtered := make([]github.ReleaseAsset, 0, len(assets))
	indexes := make([]int, 0, len(assets))
	for i, a := range assets {
		if strings.Contains(strings.ToLower(a.Name), "cc-select-desktop") {
			filtered = append(filtered, a)
			indexes = append(indexes, i)
		}
	}
	if len(filtered) == 0 {
		return -1
	}
	fi := github.DefaultAssetMatcher(req, filtered)
	if fi < 0 {
		return -1
	}
	return indexes[fi]
}

// startupUpdateCheck 启动静默检查（FR-006，prefs.autoUpdateCheck 控制）：
// 有新版本时经系统通知提醒（托盘形态下不开窗也可见）。
func startupUpdateCheck(app *application.App, notifier *notifications.NotificationService, appVersion string) {
	if appVersion == "dev" {
		return // dev 构建无版本可比
	}
	p, err := prefs.Load()
	if err != nil || !p.LoadDesktop().AutoUpdateCheck {
		return
	}
	time.Sleep(3 * time.Second) // 等应用事件循环就绪再发通知
	rel, err := app.Updater.Check(context.Background())
	if err != nil || rel == nil {
		return
	}
	_ = notifier.SendNotification(notifications.NotificationOptions{
		ID:    "update-available",
		Title: "CC Select",
		Body:  i18n.T("desktop.update.available", rel.Version),
	})
}
