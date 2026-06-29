package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cc-select/cc-select/internal/profile"
)

// TestServer_Lifecycle 覆盖 NewServer/Start/listen/actualPort 及静态文件挂载：
// 用 port=0 启动，onReady 拿到实际端口，打一次 API 请求，再 cancel ctx 优雅停机。
func TestServer_Lifecycle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CC_SELECT_CONFIG", filepath.Join(dir, "providers.json"))

	s := NewServer(0) // 0 = 系统分配空闲端口
	ctx, cancel := context.WithCancel(context.Background())

	portCh := make(chan int, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start(ctx, func(p int) { portCh <- p })
	}()

	var port int
	select {
	case port = <-portCh:
	case <-time.After(5 * time.Second):
		t.Fatal("服务未在 5s 内就绪")
	}
	if port == 0 {
		t.Fatal("actualPort 应返回系统分配的非零端口")
	}

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1/providers", port))
	if err != nil {
		t.Fatalf("请求服务失败: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /providers want 200 got %d", resp.StatusCode)
	}

	cancel() // 触发优雅停机
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Start 优雅停机应返回 nil，got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ctx 取消后服务未在 5s 内停机")
	}
}

// TestUpdateProvider_Success 覆盖 PUT 成功路径：整体覆盖 settings 并回填磁盘真值。
func TestUpdateProvider_Success(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	body := `{"name":"GLM-2","settings":{"env":{"ANTHROPIC_BASE_URL":"https://glm2"},"model":"glm-4.6"}}`
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/providers/glm",
		bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT want 200 got %d", resp.StatusCode)
	}

	// 磁盘 settings.json 应被整体覆盖为新内容。
	raw, err := profile.ReadRaw("glm")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("settings.json 非法: %v", err)
	}
	if got["model"] != "glm-4.6" {
		t.Errorf("PUT 后 model 应更新，got %v", got["model"])
	}
	env, _ := got["env"].(map[string]any)
	if env["ANTHROPIC_BASE_URL"] != "https://glm2" {
		t.Errorf("PUT 后 base url 应更新，got %v", env)
	}
}

// TestUpdateProvider_NotFound 覆盖 PUT 不存在 provider → 404。
func TestUpdateProvider_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/providers/ghost",
		strings.NewReader(`{"settings":{"env":{}}}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("PUT 不存在 provider want 404 got %d", resp.StatusCode)
	}
}

// TestCreateProvider_Conflict 覆盖 POST 重复 id → 409。
func TestCreateProvider_Conflict(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	// glm 已由 newTestServer 预置。
	body := `{"id":"glm","settings":{"env":{"ANTHROPIC_BASE_URL":"https://dup"}}}`
	resp, err := http.Post(srv.URL+"/api/v1/providers", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("POST 重复 id want 409 got %d", resp.StatusCode)
	}
}

// TestMethodNotAllowed 覆盖集合/单项路由的非法方法分支。
func TestMethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	// 集合路由不支持 DELETE。
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/providers", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("集合路由 DELETE want 405 got %d", resp.StatusCode)
	}

	// 单项路由不支持 POST。
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/v1/providers/glm", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("单项路由 POST want 405 got %d", resp.StatusCode)
	}
}

// TestCreateProvider_RejectsBadID 验证路径穿越 id（来自请求体）被 400 拒绝。
func TestCreateProvider_RejectsBadID(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()
	defer os.Unsetenv("CC_SELECT_CONFIG")

	body := `{"id":"../../evil","settings":{"env":{"ANTHROPIC_BASE_URL":"https://x"}}}`
	resp, err := http.Post(srv.URL+"/api/v1/providers", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("路径穿越 id want 400 got %d", resp.StatusCode)
	}
}

// TestHostGuard 验证 hostGuard：回环 Host 放行，其它 Host（DNS 重绑定）403。
func TestHostGuard(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CC_SELECT_CONFIG", filepath.Join(dir, "providers.json"))

	srv := httptest.NewServer(hostGuard(newAPIHandler().routes()))
	defer srv.Close()

	// 默认 Host = 127.0.0.1:port → 放行。
	resp, err := http.Get(srv.URL + "/api/v1/providers")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("回环 Host 应放行，got %d", resp.StatusCode)
	}

	// 伪造恶意 Host（模拟 DNS 重绑定）→ 403。
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/providers", nil)
	req.Host = "evil.example.com"
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("非回环 Host 应 403，got %d", resp.StatusCode)
	}
}
