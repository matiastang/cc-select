package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// T008（specs/003-desktop-client/tasks.md）：GET /api/v1/desktop/info 契约。
// 响应结构（data-model §2.1）：mode/appVersion/embeddedCliVersion + cli 三态。
// CLI 探测经接口注入——T015 提供真实实现，此处用 fake。

type fakeCliDetector struct {
	path, version string
	found         bool
}

func (f *fakeCliDetector) Detect() (string, string, bool) { return f.path, f.version, f.found }

func TestDesktopInfo_ResponseShape(t *testing.T) {
	cases := []struct {
		name    string
		det     *fakeCliDetector
		wantCli map[string]any
	}{
		{
			name: "未安装",
			det:  &fakeCliDetector{found: false},
			wantCli: map[string]any{
				"installedPath": nil, "installedVersion": nil, "updateAvailable": false,
			},
		},
		{
			name: "已装旧版可升级",
			det:  &fakeCliDetector{found: true, path: "/usr/local/bin/cc-select", version: "0.0.7"},
			wantCli: map[string]any{
				"installedPath": "/usr/local/bin/cc-select", "installedVersion": "0.0.7", "updateAvailable": true,
			},
		},
		{
			name: "已是最新",
			det:  &fakeCliDetector{found: true, path: "/usr/local/bin/cc-select", version: "0.1.0"},
			wantCli: map[string]any{
				"installedPath": "/usr/local/bin/cc-select", "installedVersion": "0.1.0", "updateAvailable": false,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newDesktopAPI("0.1.0", "0.1.0", tc.det)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/desktop/info", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			var out map[string]any
			if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
				t.Fatal(err)
			}
			if out["mode"] != "desktop" {
				t.Fatalf("mode = %v, want desktop", out["mode"])
			}
			if out["appVersion"] != "0.1.0" || out["embeddedCliVersion"] != "0.1.0" {
				t.Fatalf("versions = %v/%v, want 0.1.0", out["appVersion"], out["embeddedCliVersion"])
			}
			cli, ok := out["cli"].(map[string]any)
			if !ok {
				t.Fatalf("cli 字段缺失: %v", out)
			}
			for k, want := range tc.wantCli {
				if cli[k] != want {
					t.Fatalf("cli.%s = %v (%T), want %v", k, cli[k], cli[k], want)
				}
			}
		})
	}
}

func TestDesktopInfo_MethodNotAllowed(t *testing.T) {
	h := newDesktopAPI("0.1.0", "0.1.0", &fakeCliDetector{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/desktop/info", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST info status = %d, want 405", rec.Code)
	}
}
