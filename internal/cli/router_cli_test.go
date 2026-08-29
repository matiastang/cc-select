package cli

import (
	"net/http"
	"testing"
	"time"

	"github.com/cc-select/cc-select/internal/router"
)

// startInprocDaemon 起一个进程内真 daemon（写 router.json 到临时目录）。
func startInprocDaemon(t *testing.T) string {
	t.Helper()
	pipeline := router.ModelRewrite(router.NewForward(nil))
	s := router.NewServer("127.0.0.1:0", "test", pipeline)
	ln, err := s.Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := ln.Addr().String()
	go func() { _ = s.Serve(ln) }()
	t.Cleanup(func() { _ = s.Close() })
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := router.Probe(addr); err == nil {
			return addr
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("daemon 未就绪（%s）", addr)
	return ""
}

func TestRouterStop_NoStateIsIdempotent(t *testing.T) {
	setTempCfg(t)
	out, _, err := execRoot(t, "", "router", "stop")
	if err != nil {
		t.Fatalf("无状态文件应幂等成功: %v", err)
	}
	if !containsAny(out, "not running") {
		t.Errorf("应提示未运行: %q", out)
	}
}

func TestRouterStop_StopsRunningDaemon(t *testing.T) {
	setTempCfg(t)
	addr := startInprocDaemon(t)

	if _, _, err := execRoot(t, "", "router", "stop"); err != nil {
		t.Fatalf("stop: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := router.Probe(addr); err != nil {
			return // 已优雅退出
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("stop 未在 2s 内让 daemon 退出")
}

func TestRouterStop_WrongStateReportsError(t *testing.T) {
	setTempCfg(t)
	// 状态文件指向一个死端口（非本 daemon 的服务）。
	if err := router.SaveState(&router.State{Addr: "127.0.0.1:1", StopToken: "tok"}); err != nil {
		t.Fatal(err)
	}
	_, _, err := execRoot(t, "", "router", "stop")
	if err == nil {
		t.Error("连不上 daemon 应报错并给指引")
	}
}

// containsAny 任一子串命中即真。
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) && indexOfStr(s, sub) >= 0 {
			return true
		}
	}
	return false
}

func indexOfStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

var _ = http.MethodGet
