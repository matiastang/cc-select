package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// T014（specs/003-desktop-client/tasks.md）：POST /api/v1/desktop/cli/install 契约。
// 契约 contracts/desktop-api.md §3.3：200 结果体 / 409 冲突体（conflict 字段）/
// 400 坏 body / 405 非 POST。安装逻辑本身由 cliinstall_test.go 覆盖，此处注入 fake。

func newInstallTestAPI(install installFn) http.Handler {
	return newDesktopAPI(desktopAPIOptions{
		AppVersion:         "0.1.0",
		EmbeddedCliVersion: "0.1.0",
		Cli:                &fakeCliDetector{},
		TargetDirs:         func() []string { return []string{"/tmp/x"} },
		ExtractCli:         func(string) error { return nil },
		Install:            install,
	})
}

func postInstall(h http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/desktop/cli/install", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCliInstallEndpoint_Success(t *testing.T) {
	h := newInstallTestAPI(func(installOptions) (installResult, *installConflict, error) {
		return installResult{InstalledPath: "/usr/local/bin/cc-select", InstalledVersion: "0.1.0", Action: "installed"}, nil, nil
	})
	rec := postInstall(h, `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var out installResult
	json.NewDecoder(rec.Body).Decode(&out)
	if out.Action != "installed" || out.InstalledPath == "" {
		t.Fatalf("body = %+v", out)
	}
}

func TestCliInstallEndpoint_Conflict409(t *testing.T) {
	h := newInstallTestAPI(func(installOptions) (installResult, *installConflict, error) {
		return installResult{}, &installConflict{Kind: conflictExternal, Path: "/opt/x/cc-select", Version: "unknown"}, nil
	})
	rec := postInstall(h, `{}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	var out map[string]any
	json.NewDecoder(rec.Body).Decode(&out)
	if out["conflict"] != conflictExternal {
		t.Fatalf("body = %v, want conflict=external-binary", out)
	}
}

func TestCliInstallEndpoint_ConfirmOverwritePassesThrough(t *testing.T) {
	var got installOptions
	h := newInstallTestAPI(func(o installOptions) (installResult, *installConflict, error) {
		got = o
		return installResult{Action: "overwritten"}, nil, nil
	})
	rec := postInstall(h, `{"confirmOverwrite":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !got.ConfirmOverwrite {
		t.Fatalf("confirmOverwrite 未透传, opts=%+v", got)
	}
}

func TestCliInstallEndpoint_BadBody400(t *testing.T) {
	h := newInstallTestAPI(func(installOptions) (installResult, *installConflict, error) {
		t.Fatal("坏 body 不应触达安装器")
		return installResult{}, nil, nil
	})
	rec := postInstall(h, `{not-json`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCliInstallEndpoint_MethodNotAllowed(t *testing.T) {
	h := newInstallTestAPI(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/desktop/cli/install", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405", rec.Code)
	}
}
