package main

import (
	"net/http"
	"strings"
)

// bridge 是桌面端 AssetServer 的内容入口：在既有 web 内容 handler（API + 静态资产，
// internal/web.Handler）之上叠加桌面专属语义（specs/003-desktop-client contracts/desktop-api.md）。
//
// 分发规则（update 拦截用精确匹配，避免误吞未来路径；desktop 用命名空间前缀）：
//   - /api/v1/update/check、/api/v1/update → updateBridge（映射到 Wails Updater，T022 接入真身）
//   - /api/v1/desktop/…                   → desktopAPI（info/settings/cli/tray，T008+ 逐步接入）
//   - 其余一切                             → content 透传（既有 API 与前端静态资产零改动）
type bridge struct {
	content http.Handler
	update  http.Handler
	desktop http.Handler
}

// newBridge 组装桥接层。三个 handler 均注入，便于单测与分阶段接入。
func newBridge(content, update, desktop http.Handler) *bridge {
	return &bridge{content: content, update: update, desktop: desktop}
}

func (b *bridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch p := r.URL.Path; p {
	case "/api/v1/update/check", "/api/v1/update":
		b.update.ServeHTTP(w, r)
	default:
		if strings.HasPrefix(p, "/api/v1/desktop/") {
			b.desktop.ServeHTTP(w, r)
			return
		}
		b.content.ServeHTTP(w, r)
	}
}
