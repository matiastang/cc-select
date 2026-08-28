package router

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// netListen 在指定 addr 预占一个 TCP 监听（测试模拟 daemon 复活同 addr）。
func netListen(addr string) (net.Listener, error) { return net.Listen("tcp", addr) }

// setTempRouter 让 router.json 落入临时目录（复用 CC_SELECT_CONFIG 同级约定）。
func setTempRouter(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CC_SELECT_CONFIG", filepath.Join(dir, "providers.json"))
	t.Setenv("CC_SELECT_PROXY_ADDR", "") // 隔离外部环境
	return dir
}

func TestState_SaveLoadRoundTrip(t *testing.T) {
	setTempRouter(t)
	st := &State{Addr: "127.0.0.1:48270", PID: 42, Version: "0.0.6", StartedAt: time.Now().UTC().Truncate(time.Second)}
	if err := SaveState(st); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	got, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got.Addr != st.Addr || got.PID != st.PID || got.Version != st.Version {
		t.Errorf("往返不符: got %+v want %+v", got, st)
	}
	p, _ := StatePath()
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("router.json 权限 want 0600 got %o", fi.Mode().Perm())
	}
}

func TestLoadState_MissingReturnsNil(t *testing.T) {
	setTempRouter(t)
	got, err := LoadState()
	if err != nil || got != nil {
		t.Errorf("缺文件应返回 (nil,nil)，got (%+v,%v)", got, err)
	}
}

func TestResolveAddr_Priority(t *testing.T) {
	setTempRouter(t)
	// 1) 无状态无 env → 默认。
	if got := mustResolve(t); got != DefaultAddr {
		t.Errorf("默认 addr want %q got %q", DefaultAddr, got)
	}
	// 2) env 覆盖默认（无状态文件时）。
	t.Setenv("CC_SELECT_PROXY_ADDR", "127.0.0.1:1")
	if got := mustResolve(t); got != "127.0.0.1:1" {
		t.Errorf("env 应生效: got %q", got)
	}
	// 3) 状态文件优先于 env——已固化在 profile 里的 BASE_URL 不能漂移（研究 D7/D11）。
	if err := SaveState(&State{Addr: "127.0.0.1:2"}); err != nil {
		t.Fatal(err)
	}
	if got := mustResolve(t); got != "127.0.0.1:2" {
		t.Errorf("状态文件应优先于 env: got %q", got)
	}
}

func mustResolve(t *testing.T) string {
	t.Helper()
	addr, err := ResolveAddr()
	if err != nil {
		t.Fatalf("ResolveAddr: %v", err)
	}
	return addr
}

// healthzServer 起一个可控版本的 healthz 服务。
func healthzServer(t *testing.T, version string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": version})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestProbe_Healthy(t *testing.T) {
	srv := healthzServer(t, "0.0.6")
	h, err := Probe(hostPort(t, srv))
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if h.Status != "ok" || h.Version != "0.0.6" {
		t.Errorf("healthz 解析不符: %+v", h)
	}
}

func TestProbe_DeadPort(t *testing.T) {
	// 起一个立即关闭的端口模拟 daemon 死亡。
	srv := httptest.NewServer(http.NotFoundHandler())
	addr := hostPort(t, srv)
	srv.Close()
	if _, err := Probe(addr); err == nil {
		t.Error("死端口应返回错误")
	}
}

func hostPort(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	return srv.Listener.Addr().String()
}

func TestEnsure_HealthyReusesWithoutSpawn(t *testing.T) {
	setTempRouter(t)
	srv := healthzServer(t, "0.0.6")
	if err := SaveState(&State{Addr: hostPort(t, srv), Version: "0.0.6"}); err != nil {
		t.Fatal(err)
	}
	spawned := false
	deps := EnsureDeps{Spawn: func(string) error { spawned = true; return nil }, SelfVersion: "0.0.6"}
	addr, err := deps.Ensure()
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if addr != hostPort(t, srv) {
		t.Errorf("应复用既有 addr: got %q", addr)
	}
	if spawned {
		t.Error("健康时不应重新拉起")
	}
}

func TestEnsure_DeadRespawns(t *testing.T) {
	setTempRouter(t)
	// 预占一个端口但不服务任何 HTTP：入口探测必然失败（模拟 daemon 已死）。
	ln, err := netListen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("预占失败: %v", err)
	}
	deadAddr := ln.Addr().String()
	if err := SaveState(&State{Addr: deadAddr, Version: "0.0.6"}); err != nil {
		t.Fatal(err)
	}

	spawnCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": "0.0.6"})
	})
	// spawn = 在同 addr 上复活服务（生产中是 detached serve，语义等价：同 addr 复活）。
	deps := EnsureDeps{
		Spawn: func(string) error {
			spawnCount++
			srv := &httptest.Server{Listener: ln, Config: &http.Server{Handler: mux}}
			srv.Start()
			t.Cleanup(srv.Close)
			return nil
		},
		SelfVersion: "0.0.6",
		Wait:        2 * time.Second,
	}
	addr, err := deps.Ensure()
	if err != nil {
		t.Fatalf("Ensure 应自愈: %v", err)
	}
	if addr != deadAddr {
		t.Errorf("复活必须沿用原 addr（研究 D7）: got %q", addr)
	}
	if spawnCount != 1 {
		t.Errorf("应恰好 spawn 一次，got %d", spawnCount)
	}
}

func TestEnsure_VersionMismatchRespawns(t *testing.T) {
	setTempRouter(t)
	var ver atomic.Value
	ver.Store("0.0.5")
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": ver.Load().(string)})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	if err := SaveState(&State{Addr: hostPort(t, srv), Version: "0.0.5"}); err != nil {
		t.Fatal(err)
	}
	spawnCount := 0
	deps := EnsureDeps{
		Spawn:       func(string) error { spawnCount++; ver.Store("0.0.6"); return nil },
		SelfVersion: "0.0.6",
		Wait:        2 * time.Second,
	}
	if _, err := deps.Ensure(); err != nil {
		t.Fatalf("版本不匹配应触发换新: %v", err)
	}
	if spawnCount != 1 {
		t.Errorf("旧版本 daemon 应被替换，spawn=%d", spawnCount)
	}
}

func TestEnsure_SpawnFailReturnsError(t *testing.T) {
	setTempRouter(t)
	deps := EnsureDeps{
		Spawn:       func(string) error { return os.ErrPermission },
		SelfVersion: "0.0.6",
		Wait:        500 * time.Millisecond,
	}
	if _, err := deps.Ensure(); err == nil {
		t.Error("spawn 失败应返回错误（调用方给恢复指引）")
	}
}

func TestEnsure_TimeoutWhenNeverHealthy(t *testing.T) {
	setTempRouter(t)
	dead := httptest.NewServer(http.NotFoundHandler())
	addr := hostPort(t, dead)
	dead.Close()
	if err := SaveState(&State{Addr: addr}); err != nil {
		t.Fatal(err)
	}
	deps := EnsureDeps{
		Spawn:       func(string) error { return nil }, // 拉起但永远不健康
		SelfVersion: "0.0.6",
		Wait:        300 * time.Millisecond,
	}
	start := time.Now()
	if _, err := deps.Ensure(); err == nil {
		t.Error("超时应报错")
	} else if time.Since(start) > 2*time.Second {
		t.Error("应尊重 Wait 预算，及时失败")
	}
}
