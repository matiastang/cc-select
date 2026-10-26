package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/prefs"
)

// T021（specs/003-desktop-client/tasks.md）：GET/PUT /api/v1/desktop/settings 契约
//（contracts/desktop-api.md §3.2）：GET 归一默认；PUT 204 幂等、缺字段 400、
// 服务同步失败 503 且 prefs 不落盘（先服务后落盘，整体回滚）。

type fakePrefsStore struct {
	p       *prefs.Prefs
	loadErr error
	saveErr error
	saved   []*prefs.Prefs
}

func (s *fakePrefsStore) Load() (*prefs.Prefs, error) {
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	if s.p == nil {
		return &prefs.Prefs{}, nil
	}
	cp := *s.p
	return &cp, nil
}
func (s *fakePrefsStore) Save(pr *prefs.Prefs) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	cp := *pr
	s.saved = append(s.saved, &cp)
	s.p = &cp
	return nil
}

type fakeServiceSync struct {
	err   error
	calls []prefs.DesktopPrefs
}

func (f *fakeServiceSync) sync(d prefs.DesktopPrefs) error {
	f.calls = append(f.calls, d)
	return f.err
}

func newSettingsAPI(store *fakePrefsStore, sync *fakeServiceSync) http.Handler {
	return newDesktopAPI(desktopAPIOptions{
		AppVersion:         "0.1.0",
		EmbeddedCliVersion: "0.1.0",
		Cli:                &fakeCliDetector{},
		TargetDirs:         func() []string { return nil },
		ExtractCli:         func(string) error { return nil },
		LoadPrefs:          store.Load,
		SavePrefs:          store.Save,
		SyncServices:       sync.sync,
	})
}

func TestSettings_GetDefaults(t *testing.T) {
	h := newSettingsAPI(&fakePrefsStore{}, &fakeServiceSync{})
	rec := doReq(h, http.MethodGet, "/api/v1/desktop/settings")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var out map[string]any
	json.NewDecoder(rec.Body).Decode(&out)
	if out["trayResident"] != true || out["autoStart"] != false || out["autoUpdateCheck"] != true {
		t.Fatalf("defaults = %v", out)
	}
}

func TestSettings_PutRoundTrip(t *testing.T) {
	store := &fakePrefsStore{}
	h := newSettingsAPI(store, &fakeServiceSync{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/desktop/settings",
		strings.NewReader(`{"trayResident":false,"autoStart":true,"autoUpdateCheck":false}`)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("PUT status = %d, body=%s", rec.Code, rec.Body.String())
	}
	// GET 反映新值。
	rec2 := doReq(h, http.MethodGet, "/api/v1/desktop/settings")
	var out map[string]any
	json.NewDecoder(rec2.Body).Decode(&out)
	if out["trayResident"] != false || out["autoStart"] != true || out["autoUpdateCheck"] != false {
		t.Fatalf("round-trip = %v", out)
	}
	// 幂等：重复 PUT 相同 body 仍 204。
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, httptest.NewRequest(http.MethodPut, "/api/v1/desktop/settings",
		strings.NewReader(`{"trayResident":false,"autoStart":true,"autoUpdateCheck":false}`)))
	if rec3.Code != http.StatusNoContent {
		t.Fatalf("idempotent PUT = %d", rec3.Code)
	}
}

func TestSettings_PutMissingField400(t *testing.T) {
	store := &fakePrefsStore{}
	h := newSettingsAPI(store, &fakeServiceSync{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/desktop/settings",
		strings.NewReader(`{"trayResident":false}`))) // 缺 autoStart/autoUpdateCheck
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("缺字段应 400, got %d", rec.Code)
	}
	if len(store.saved) != 0 {
		t.Fatal("400 时不得落盘")
	}
}

func TestSettings_ServiceSyncFailure503AndRollback(t *testing.T) {
	store := &fakePrefsStore{
		p: &prefs.Prefs{Desktop: &prefs.DesktopPrefs{TrayResident: true, AutoStart: false, AutoUpdateCheck: true}},
	}
	h := newSettingsAPI(store, &fakeServiceSync{err: errString("autostart unavailable")})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/desktop/settings",
		strings.NewReader(`{"trayResident":true,"autoStart":true,"autoUpdateCheck":true}`)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("服务同步失败应 503, got %d", rec.Code)
	}
	if len(store.saved) != 0 {
		t.Fatal("503 时 prefs 不得落盘（整体回滚，UI 与实际一致）")
	}
}

func TestSettings_MethodGuard(t *testing.T) {
	h := newSettingsAPI(&fakePrefsStore{}, &fakeServiceSync{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/desktop/settings", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE = %d, want 405", rec.Code)
	}
}
