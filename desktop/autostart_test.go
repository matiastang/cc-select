package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/prefs"
)

// T031（specs/003-desktop-client/tasks.md）：自启服务同步适配器。
// 契约：autoStart=true → Enable；false → Disable；服务失败 → 包装错误
//（settings 端点据此 503 且 prefs 不落盘——回滚语义已在 settings_test.go 覆盖，
// 此处锁定适配器自身的行为与端到端 503 联动）。

type fakeAutostart struct {
	enableErr  error
	disableErr error
	enabled    []bool // 记录 Enable/Disable 调用序列
}

func (f *fakeAutostart) Enable() error  { f.enabled = append(f.enabled, true); return f.enableErr }
func (f *fakeAutostart) Disable() error { f.enabled = append(f.enabled, false); return f.disableErr }

func TestAutostartSync_EnableDisableMapping(t *testing.T) {
	fa := &fakeAutostart{}
	sync := autostartSync(fa)

	if err := sync(prefs.DesktopPrefs{AutoStart: true}); err != nil {
		t.Fatalf("enable 不应报错: %v", err)
	}
	if err := sync(prefs.DesktopPrefs{AutoStart: false}); err != nil {
		t.Fatalf("disable 不应报错: %v", err)
	}
	if len(fa.enabled) != 2 || fa.enabled[0] != true || fa.enabled[1] != false {
		t.Fatalf("调用序列 = %v, want [true false]", fa.enabled)
	}
}

func TestAutostartSync_FailurePropagatesAndRollsBackPrefs(t *testing.T) {
	fa := &fakeAutostart{enableErr: errString("managed by policy")}
	store := &fakePrefsStore{}
	h := newDesktopAPI(desktopAPIOptions{
		AppVersion:         "0.1.0",
		EmbeddedCliVersion: "0.1.0",
		Cli:                &fakeCliDetector{},
		TargetDirs:         func() []string { return nil },
		ExtractCli:         func(string) error { return nil },
		LoadPrefs:          store.Load,
		SavePrefs:          store.Save,
		SyncServices:       autostartSync(fa),
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/desktop/settings",
		strings.NewReader(`{"trayResident":true,"autoStart":true,"autoUpdateCheck":true}`)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("自启失败应 503, got %d", rec.Code)
	}
	if len(store.saved) != 0 {
		t.Fatal("prefs 不得落盘（UI 与实际一致）")
	}
}
