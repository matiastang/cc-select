package main

import (
	"encoding/json"
	"net/http"

	"github.com/cc-select/cc-select/internal/prefs"
)

// 桌面偏好端点（US3 起，FR-008/FR-010 开关的读写面；契约 §3.2）。
// 语义：PUT 全量对象（三布尔缺一 400）；先 SyncServices 后落盘——服务同步
// 失败整体回滚不落盘（UI 与实际状态一致）；GET 读侧归一默认。

// settingsBody 用指针字段区分「显式 false」与「缺失」（缺失即 400）。
type settingsBody struct {
	TrayResident    *bool `json:"trayResident"`
	AutoStart       *bool `json:"autoStart"`
	AutoUpdateCheck *bool `json:"autoUpdateCheck"`
}

// handleSettings 按方法分发（ServeMux 精确路径不可重复注册）。
func (a *desktopAPI) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleSettingsGet(w, r)
	case http.MethodPut:
		a.handleSettingsPut(w, r)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *desktopAPI) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	p, err := a.LoadPrefs()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONBody(w, http.StatusOK, p.LoadDesktop())
}

func (a *desktopAPI) handleSettingsPut(w http.ResponseWriter, r *http.Request) {
	var body settingsBody
	// DisallowUnknownFields：未知字段 400（契约 §3.2），防拼写错误被静默吞掉。
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil ||
		body.TrayResident == nil || body.AutoStart == nil || body.AutoUpdateCheck == nil {
		writeJSONError(w, http.StatusBadRequest, "invalid settings body: all three fields required")
		return
	}
	next := prefs.DesktopPrefs{
		TrayResident:    *body.TrayResident,
		AutoStart:       *body.AutoStart,
		AutoUpdateCheck: *body.AutoUpdateCheck,
	}

	// 先服务后落盘：autostart/tray 等 OS 层注册失败 → 503，prefs 不变更。
	if a.SyncServices != nil {
		if err := a.SyncServices(next); err != nil {
			writeJSONError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
	}

	p, err := a.LoadPrefs()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	p.Desktop = &next
	if err := a.SavePrefs(p); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
