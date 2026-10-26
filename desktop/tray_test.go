package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cc-select/cc-select/internal/config"
)

// T027（specs/003-desktop-client/tasks.md）：托盘数据端点契约
//（contracts/desktop-api.md §3.4）：providers 列表 → copyCommand；**不存在任何
//「执行切换」动作**——FR-009 的契约层锁定（research R5：托盘切换=复制命令）。

func newTrayAPI(providers []config.Provider, loadErr error) http.Handler {
	return newDesktopAPI(desktopAPIOptions{
		AppVersion:         "0.1.0",
		EmbeddedCliVersion: "0.1.0",
		Cli:                &fakeCliDetector{},
		TargetDirs:         func() []string { return nil },
		ExtractCli:         func(string) error { return nil },
		ListProviders: func() ([]config.Provider, error) {
			return providers, loadErr
		},
	})
}

func TestTrayCommands_ResponseShape(t *testing.T) {
	h := newTrayAPI([]config.Provider{
		{ID: "glm", Name: "GLM"},
		{ID: "minimax", Name: "MiniMax"},
	}, nil)
	rec := doReq(h, http.MethodGet, "/api/v1/desktop/tray/commands")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var out []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("want 2 commands, got %d", len(out))
	}
	if out[0]["providerId"] != "glm" || out[0]["label"] != "GLM" || out[0]["copyCommand"] != "ccs use glm" {
		t.Fatalf("first = %v", out[0])
	}
	// 契约红线：响应里不得出现任何执行切换类字段（switch/route/apply）。
	b, _ := json.Marshal(out)
	for _, banned := range []string{"switch", "route", "apply", "activate"} {
		if bytes.Contains(b, []byte(banned)) {
			t.Fatalf("托盘命令响应不得含 %q 字段（FR-009）: %s", banned, b)
		}
	}
}

func TestTrayCommands_EmptyAndError(t *testing.T) {
	// 无 provider → 空数组（非 null），托盘菜单渲染"无 provider"占位。
	rec := doReq(newTrayAPI(nil, nil), http.MethodGet, "/api/v1/desktop/tray/commands")
	var out []map[string]any
	json.NewDecoder(rec.Body).Decode(&out)
	if out == nil || len(out) != 0 {
		t.Fatalf("want empty array, got %v", out)
	}

	// 读取失败 → 500 JSON 错误。
	rec2 := doReq(newTrayAPI(nil, errString("boom")), http.MethodGet, "/api/v1/desktop/tray/commands")
	if rec2.Code != http.StatusInternalServerError {
		t.Fatalf("load error = %d, want 500", rec2.Code)
	}
}

func TestTrayCommands_MethodGuard(t *testing.T) {
	rec := doReq(newTrayAPI(nil, nil), http.MethodPost, "/api/v1/desktop/tray/commands")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d, want 405", rec.Code)
	}
}
