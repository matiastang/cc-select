package main

import (
	_embed "embed"
	"net/http"
	"runtime"
	"sort"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/icons"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

var _ = _embed.FS{} // 保持 embed 导入（trayIconPNG 的 //go:embed 指令需要 []byte 形式）

// 托盘（US4，FR-008/FR-009；research R5 语义）：
//   - providers 菜单项点击 = 复制 `ccs use <id>` 到剪贴板 + 系统通知确认——
//     不做任何「执行切换」（R1 红线：托盘没有合法的全局切换语义）
//   - 打开面板（Show 主窗口）/ 退出
//   - trayResident=false 时关窗即退出（进程结束、托盘消失）
//
// 菜单数据另有只读端点 GET /api/v1/desktop/tray/commands（契约 §3.4，可单测）。

//go:embed build/appicon.png
var trayIconPNG []byte

// trayCommandDTO 是 GET /api/v1/desktop/tray/commands 的响应项（契约 §3.4）。
type trayCommandDTO struct {
	ProviderID  string `json:"providerId"`
	Label       string `json:"label"`
	CopyCommand string `json:"copyCommand"`
}

// handleTrayCommands 只读端点：providers → copyCommand（FR-009 契约层锁定：
// 响应不含任何执行切换类动作字段）。
func (a *desktopAPI) handleTrayCommands(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	providers, err := a.ListProviders()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]trayCommandDTO, 0, len(providers)) // 空 slice 非 nil（空数组而非 null）
	for _, p := range providers {
		out = append(out, trayCommandDTO{
			ProviderID:  p.ID,
			Label:       p.DisplayName(),
			CopyCommand: "ccs use " + p.ID,
		})
	}
	writeJSONBody(w, http.StatusOK, out)
}

// sortedProviders 从 config 读取并按 ID 排序（托盘菜单顺序稳定）。
func sortedProviders() ([]config.Provider, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	out := make([]config.Provider, 0, len(cfg.Providers))
	for _, p := range cfg.Providers {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// trayController 装配系统托盘；菜单在关键时机重建（读时拉取，spec Edge Case
// 「运行中配置变更可见」的近似实现：启动/关窗驻留/打开面板时刷新）。
type trayController struct {
	app      *application.App
	tray     *application.SystemTray
	window   *application.WebviewWindow
	notifier *notifications.NotificationService
}

func newTrayController(app *application.App, window *application.WebviewWindow, notifier *notifications.NotificationService) *trayController {
	tray := app.SystemTray.New()
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(icons.SystrayMacTemplate)
	} else {
		tray.SetIcon(trayIconPNG)
	}
	tc := &trayController{app: app, tray: tray, window: window, notifier: notifier}
	tc.rebuildMenu()
	return tc
}

func (tc *trayController) rebuildMenu() {
	menu := tc.app.NewMenu()
	providers, err := sortedProviders()
	if err != nil || len(providers) == 0 {
		menu.AddSectionHeader(i18n.T("desktop.tray.noProviders"))
	} else {
		menu.AddSectionHeader(i18n.T("desktop.tray.copyHeader"))
		for _, p := range providers {
			item := menu.Add(p.DisplayName() + "  ·  ccs use " + p.ID)
			id, label := p.ID, p.DisplayName()
			item.OnClick(func(*application.Context) {
				// R1 红线下的托盘语义：只复制命令，不执行任何切换。
				tc.app.Clipboard.SetText("ccs use " + id)
				_ = tc.notifier.SendNotification(notifications.NotificationOptions{
					ID:    "tray-copy-" + id,
					Title: "CC Select",
					Body:  i18n.T("desktop.tray.copied", label),
				})
			})
		}
	}
	menu.AddSeparator()
	open := menu.Add(i18n.T("desktop.tray.open"))
	open.OnClick(func(*application.Context) {
		tc.window.Show()
		tc.window.Focus()
		tc.rebuildMenu() // 打开面板时顺带刷新菜单（读时拉取）
	})
	quit := menu.Add(i18n.T("desktop.tray.quit"))
	quit.OnClick(func(*application.Context) { tc.app.Quit() })

	tc.tray.SetMenu(menu)
}

// onWindowClosing 实现关窗语义（FR-008）：resident → 隐藏驻留（取消关闭）；
// 否则退出整个 App（进程结束、托盘消失，spec Story 4 验收 4）。
func (tc *trayController) onWindowClosing(e *application.WindowEvent) {
	p, err := prefs.Load()
	// prefs 读取失败时 fail-open 驻留（默认语义）：误退出比误驻留更伤用户数据面。
	if err != nil || p.LoadDesktop().TrayResident {
		tc.window.Hide()
		e.Cancel()
		tc.rebuildMenu() // 驻留时机刷新菜单（覆盖窗口打开期间的配置变更）
		return
	}
	tc.app.Quit()
}
