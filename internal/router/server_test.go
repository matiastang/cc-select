package router

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cc-select/cc-select/internal/routes"
)

// startTestServer 起一个真实监听的 daemon（随机 loopback 端口），返回 addr。
func startTestServer(t *testing.T, forward http.Handler) (addr string, s *Server) {
	t.Helper()
	setTempRouter(t)
	s = NewServer("127.0.0.1:0", "0.0.6", forward)
	ln, err := s.Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr = ln.Addr().String()
	if ip := ln.Addr().(*net.TCPAddr).IP; !ip.IsLoopback() {
		t.Fatalf("必须仅绑定 loopback，got %v", ip)
	}
	go func() { _ = s.Serve(ln) }()
	t.Cleanup(func() { _ = s.Close() })
	// 等 healthz 就绪。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := Probe(addr); err == nil {
			return addr, s
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server 未在 2s 内就绪（%s）", addr)
	return "", nil
}

func doGet(t *testing.T, url, bearer string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求 %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestServer_HealthzAndState(t *testing.T) {
	addr, _ := startTestServer(t, http.NotFoundHandler())

	code, body := doGet(t, "http://"+addr+"/healthz", "")
	if code != http.StatusOK {
		t.Fatalf("healthz want 200 got %d: %s", code, body)
	}
	var h HealthInfo
	if err := json.Unmarshal([]byte(body), &h); err != nil || h.Status != "ok" || h.Version != "0.0.6" {
		t.Errorf("healthz 体不符: %s", body)
	}

	// Listen 应写状态文件（addr/pid/version/stopToken）。
	st, err := LoadState()
	if err != nil || st == nil {
		t.Fatalf("应已写状态文件: %+v %v", st, err)
	}
	if st.Addr != addr || st.PID == 0 || st.Version != "0.0.6" || st.StopToken == "" {
		t.Errorf("状态文件字段不全: %+v", st)
	}
}

func TestServer_AuthGate(t *testing.T) {
	var hit bool
	forward := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("upstream"))
	})
	addr, _ := startTestServer(t, forward)

	tid, _ := routes.NewTID()
	if err := routes.Set(tid, "glm"); err != nil {
		t.Fatalf("准备路由: %v", err)
	}
	unknownTID := "ccs-00000000000000000000000000000000"

	// 1) 无 Bearer → 401 且带人类可读指引。
	code, body := doGet(t, "http://"+addr+"/v1/messages", "")
	if code != http.StatusUnauthorized {
		t.Errorf("无 token want 401 got %d", code)
	}
	if !containsBody(body, "ccs use") {
		t.Errorf("401 体应含指引: %s", body)
	}
	// 2) 未知 tid → 401。
	if code, _ = doGet(t, "http://"+addr+"/v1/messages", unknownTID); code != http.StatusUnauthorized {
		t.Errorf("未知 tid want 401 got %d", code)
	}
	if hit {
		t.Error("未鉴权请求不得触达转发层")
	}
	// 3) 已知 tid → 放行到转发层。
	if code, body = doGet(t, "http://"+addr+"/v1/messages", tid); code != http.StatusOK || body != "upstream" {
		t.Errorf("已知 tid 应转发: code=%d body=%s", code, body)
	}
	if !hit {
		t.Error("已鉴权请求应触达转发层")
	}
}

func TestServer_StopSideChannel(t *testing.T) {
	addr, s := startTestServer(t, http.NotFoundHandler())
	st, _ := LoadState()

	// 错误 token → 拒绝且进程存活。
	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/healthz", nil)
	req.Header.Set(stopHeader, "wrong-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("stop 请求: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("错误 stop token want 403 got %d", resp.StatusCode)
	}
	if _, err := Probe(addr); err != nil {
		t.Fatalf("错误 token 不应杀死 daemon: %v", err)
	}

	// 正确 token → 优雅退出（探活失败即确认已停）。
	req2, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/healthz", nil)
	req2.Header.Set(stopHeader, st.StopToken)
	if resp, err := http.DefaultClient.Do(req2); err != nil {
		t.Fatalf("stop 请求: %v", err)
	} else {
		resp.Body.Close()
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := Probe(addr); err != nil {
			return // 已停止
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("stop 未在 2s 内生效")
	_ = s
}

// containsBody 断言 body 是否含子串。
func containsBody(body, sub string) bool {
	return strings.Contains(body, sub)
}
