package main

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// 自启服务同步（US5，FR-010；research R6 用 v3 内置 AutostartManager——
// mac 登录项 / win 注册表 / linux XDG autostart 的跨平台封装）。

// autostartService 是自启服务的最小接口（真身 *application.AutostartManager；测试 fake）。
type autostartService interface {
	Enable() error
	Disable() error
}

// 编译期断言：Wails 内置管理器满足注入缝。
var _ autostartService = (*application.AutostartManager)(nil)

// autostartSync 把 DesktopPrefs.AutoStart 映射到 OS 自启注册。
// 失败返回错误 → settings 端点 503 且 prefs 不落盘（整体回滚，UI 与实际一致）。
func autostartSync(svc autostartService) func(prefs.DesktopPrefs) error {
	return func(d prefs.DesktopPrefs) error {
		var err error
		if d.AutoStart {
			err = svc.Enable()
		} else {
			err = svc.Disable()
		}
		if err != nil {
			return fmt.Errorf("autostart unavailable: %w", err)
		}
		return nil
	}
}

// autostartHolder 解决"settings 端点构造先于 app.Autostart 存在"的次序问题
// （同 updaterHolder）。svc 未绑定的极早期窗口内调用视为无操作，不阻断。
type autostartHolder struct {
	svc autostartService
}

func (h *autostartHolder) sync(d prefs.DesktopPrefs) error {
	if h.svc == nil {
		return nil
	}
	return autostartSync(h.svc)(d)
}
