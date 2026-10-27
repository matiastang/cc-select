package main

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

// 更新桥（US3，FR-006/FR-007；契约 contracts/desktop-api.md §2）。
// 拦截 /api/v1/update*，把语义映射到 Wails Updater（headless）：前端
// CheckUpdateButton / 更新卡片零改动（响应与 web 端点同构）。
//
// 红线（research R2）：桌面模式绝不调用 internal/updater 的二进制同路径替换——
// 那会替换桌面 App 自身路径的假设不成立。

// updaterService 是 Wails Updater 的注入缝（真身 *updater.Updater；测试 fake）。
type updaterService interface {
	Check(ctx context.Context) (*updater.Release, error)
	DownloadAndInstall(ctx context.Context) error
}

type updateBridge struct {
	appVersion string
	svc        updaterService
	mu         sync.Mutex // 串行化安装点击（与 web 端点 updateMu 同语义）
}

// newUpdateBridge 组装更新桥。svc 通常传 app.Updater（Init 之后的实例）。
func newUpdateBridge(appVersion string, svc updaterService) http.Handler {
	b := &updateBridge{appVersion: appVersion, svc: svc}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/update/check", b.handleCheck)
	mux.HandleFunc("/api/v1/update", b.handleInstall)
	return mux
}

func (b *updateBridge) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// 与 web 端点同构的字段（internal/web/api.go handleUpdateCheck）。
	out := map[string]any{
		"currentVersion": b.appVersion,
		"latestVersion":  b.appVersion,
		"hasUpdate":      false,
		"devBuild":       b.appVersion == "dev",
	}
	if rel, err := b.svc.Check(r.Context()); err == nil && rel != nil {
		out["latestVersion"] = rel.Version
		out["hasUpdate"] = true
		out["assetName"] = rel.Artifact.Filename
		out["releaseNotes"] = rel.Notes
		if u, ok := rel.Metadata["html_url"].(string); ok {
			out["htmlUrl"] = u
		}
	} else if err != nil {
		// 温和降级：不 5xx 打断界面（FR-006 验收 3）。
		out["error"] = "check failed"
	}
	writeJSONBody(w, http.StatusOK, out)
}

func (b *updateBridge) handleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !b.mu.TryLock() {
		writeJSONError(w, http.StatusConflict, "update in progress")
		return
	}
	defer b.mu.Unlock()

	// 安装不随 HTTP 请求取消而中断（webview 导航/断开不应砍掉下载与校验），
	// 用独立超时上下文（下载大包 + 摘要校验的合理上限）。
	installCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := b.svc.DownloadAndInstall(installCtx); err != nil {
		msg := err.Error()
		// 校验类失败（摘要/签名不匹配）422：当前版本完好（FR-007 验收 2）。
		if strings.Contains(strings.ToLower(msg), "checksum") ||
			strings.Contains(strings.ToLower(msg), "digest") ||
			strings.Contains(strings.ToLower(msg), "signature") {
			writeJSONError(w, http.StatusUnprocessableEntity, msg)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, msg)
		return
	}

	rel, _ := b.svc.Check(r.Context())
	to := ""
	if rel != nil {
		to = rel.Version
	}
	// 与 web 端点同构的成功体：status=installed + restartRequired——
	// 桌面形态下新版本经 Wails Updater 原子替换，重启 App 生效。
	writeJSONBody(w, http.StatusOK, map[string]any{
		"status":          "installed",
		"fromVersion":     b.appVersion,
		"toVersion":       to,
		"restartRequired": true,
		"message":         "update staged; restart the app to apply",
	})
}
