package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// T005（specs/003-desktop-client/tasks.md）：导出 Handler() 供桌面端 AssetServer 复用。
// 契约：同一 handler 同时伺服 API（/api/v1/*）与前端静态资产（/），不含 hostGuard
//（桌面 WebView origin 为 wails.localhost，且无 TCP 监听即无 DNS 重绑定向量）。

func TestHandler_ServesAPIAndStaticAssets(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "providers.json")
	os.Setenv("CC_SELECT_CONFIG", cfg)
	defer os.Unsetenv("CC_SELECT_CONFIG")
	_ = os.WriteFile(cfg, []byte(`{"providers":{"glm":{"id":"glm","name":"GLM"}}}`), 0o600)

	srv := httptest.NewServer(Handler())
	defer srv.Close()

	// 静态资产：/ 返回前端 index.html。
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", resp.StatusCode)
	}
	body := make([]byte, 512)
	n, _ := resp.Body.Read(body)
	if !strings.Contains(strings.ToLower(string(body[:n])), "<!doctype html") {
		t.Fatalf("GET / body should be index.html, got: %q", string(body[:n]))
	}

	// API：/api/v1/providers 返回 provider 列表 JSON。
	resp2, err := http.Get(srv.URL + "/api/v1/providers")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/providers status = %d, want 200", resp2.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&out); err != nil {
		t.Fatalf("decode providers: %v", err)
	}
	if _, ok := out["providers"]; !ok {
		t.Fatalf("providers response missing 'providers' field: %v", out)
	}
}
