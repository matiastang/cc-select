package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// T007（specs/003-desktop-client/tasks.md）：桥接层路由核心。
// 契约（contracts/desktop-api.md §1，按 beta.34 AssetOptions.Handler 服务全部内容的事实修订）：
//   - 精确拦截 /api/v1/update/check 与 /api/v1/update → 桌面更新桥（T022 实现真身）
//   - /api/v1/desktop/ 前缀 → 桌面专属端点（T008+）
//   - 其余一切（既有 API 与静态资产）→ 透传内容 handler
//   - update 拦截用精确匹配：/api/v1/updatex 等未来路径不被误吞

// markerHandler 记录命中的 handler 名，便于断言分发目标。
type markerHandler struct {
	name  string
	hits  int
	reply string
}

func (m *markerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.hits++
	w.Write([]byte(m.name))
}

func newTestBridge() (*bridge, *markerHandler, *markerHandler, *markerHandler) {
	content := &markerHandler{name: "content"}
	update := &markerHandler{name: "update"}
	desktop := &markerHandler{name: "desktop"}
	return newBridge(content, update, desktop), content, update, desktop
}

func TestBridge_RoutesUpdatePathsToUpdateBridge(t *testing.T) {
	b, _, update, _ := newTestBridge()
	for _, path := range []string{"/api/v1/update/check", "/api/v1/update"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		b.ServeHTTP(rec, req)
		if update.hits != 1 {
			t.Fatalf("%s should dispatch to update bridge", path)
		}
		update.hits = 0
	}
}

func TestBridge_RoutesDesktopPrefixToDesktopHandler(t *testing.T) {
	b, _, _, desktop := newTestBridge()
	for _, path := range []string{"/api/v1/desktop/info", "/api/v1/desktop/settings", "/api/v1/desktop/cli/install", "/api/v1/desktop/tray/commands"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		b.ServeHTTP(rec, req)
		if desktop.hits != 1 {
			t.Fatalf("%s should dispatch to desktop endpoints", path)
		}
		desktop.hits = 0
	}
}

func TestBridge_PassesThroughContentPaths(t *testing.T) {
	b, content, update, desktop := newTestBridge()
	paths := []string{
		"/", "/index.html", "/assets/index-abc.js",
		"/api/v1/providers", "/api/v1/providers/glm", "/api/v1/mode",
		"/api/v1/updatex", // 精确匹配：非 update 端点不误吞
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		b.ServeHTTP(rec, req)
		if content.hits != 1 {
			t.Fatalf("%s should pass through to content handler", path)
		}
		if update.hits != 0 || desktop.hits != 0 {
			t.Fatalf("%s should not touch update/desktop handlers", path)
		}
		content.hits = 0
	}
}
