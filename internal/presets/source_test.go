package presets

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 构造一个合法但与 embed 不同的 bundle（多一个 marker 预设），用于断言 store 被热替换。
func remoteBundleJSON(t *testing.T) []byte {
	t.Helper()
	b := validBundle()
	b["presets"] = append(
		b["presets"].([]map[string]any),
		map[string]any{
			"id":           "remote-marker",
			"displayName":  "Remote Marker",
			"category":     "third_party",
			"apiFormat":    "anthropic",
			"authField":    "ANTHROPIC_AUTH_TOKEN",
			"envTemplate":  map[string]string{"ANTHROPIC_BASE_URL": "https://marker.example.com", "ANTHROPIC_AUTH_TOKEN": "${API_KEY}"},
			"requiredVars": []string{"ANTHROPIC_AUTH_TOKEN"},
			"oauth":        false,
		},
	)
	return marshal(t, b)
}

// 合法最小预设的 JSON 片段（缓存测试夹具）。
const valid_preset = `{"id":"cached-marker","displayName":"Cached","category":"third_party","apiFormat":"anthropic","authField":"ANTHROPIC_AUTH_TOKEN","envTemplate":{"ANTHROPIC_BASE_URL":"https://c.example.com","ANTHROPIC_AUTH_TOKEN":"${API_KEY}"},"requiredVars":["ANTHROPIC_AUTH_TOKEN"],"oauth":false}`

type testOpts struct {
	opts ResolveOptions
}

func newTestOpts(t *testing.T, url string) testOpts {
	t.Helper()
	// 测试隔离：重置进程内解析状态；用例结束后恢复内置 bundle，避免污染其他测试。
	resolveState.mu.Lock()
	resolveState.done = false
	resolveState.result = SourceMeta{}
	resolveState.mu.Unlock()
	t.Cleanup(func() {
		var b Bundle
		_ = json.Unmarshal(embeddedPresets, &b)
		reloadStore(b)
		resolveState.mu.Lock()
		resolveState.done = false
		resolveState.result = SourceMeta{}
		resolveState.mu.Unlock()
	})
	dir := t.TempDir()
	return testOpts{opts: ResolveOptions{
		RemoteURL: url,
		CachePath: filepath.Join(dir, "presets.cache.json"),
		Client:    &http.Client{Timeout: 2 * time.Second},
		Now:       func() time.Time { return time.Now() },
	}}
}

func TestResolve_FetchesRemoteAndSwapsStore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(remoteBundleJSON(t))
	}))
	defer srv.Close()

	to := newTestOpts(t, srv.URL)
	meta := EnsureResolved(context.Background(), &to.opts)
	if meta.Source != SourceRemote {
		t.Fatalf("source=%s, want remote", meta.Source)
	}
	if meta.FetchedAt == nil {
		t.Fatal("FetchedAt should be set for remote source")
	}
	if _, ok := ByID("remote-marker"); !ok {
		t.Fatal("remote preset should be live in store")
	}
	// 缓存文件落盘
	if _, err := os.Stat(to.opts.CachePath); err != nil {
		t.Fatalf("cache file not written: %v", err)
	}
	// 进程内二次解析不再重复拉取
	meta2 := EnsureResolved(context.Background(), &to.opts)
	if meta2.Source != SourceRemote {
		t.Fatalf("second resolve source=%s, want remote (in-memory)", meta2.Source)
	}
}

func TestResolve_FallsBackToBuiltinOnUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	to := newTestOpts(t, srv.URL)
	meta := EnsureResolved(context.Background(), &to.opts)
	if meta.Source != SourceBuiltin {
		t.Fatalf("source=%s, want builtin", meta.Source)
	}
	if _, ok := ByID("remote-marker"); ok {
		t.Fatal("store must stay builtin")
	}
}

func TestResolve_FallsBackToBuiltinOnInvalidSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"schemaVersion":1,"presets":[{"id":"x"}]}`)) // 缺字段 → 整包拒绝
	}))
	defer srv.Close()

	to := newTestOpts(t, srv.URL)
	meta := EnsureResolved(context.Background(), &to.opts)
	if meta.Source != SourceBuiltin {
		t.Fatalf("source=%s, want builtin", meta.Source)
	}
}

func TestResolve_DisabledUsesBuiltinWithoutNetwork(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = w.Write(remoteBundleJSON(t))
	}))
	defer srv.Close()

	to := newTestOpts(t, srv.URL)
	to.opts.Disabled = true
	meta := EnsureResolved(context.Background(), &to.opts)
	if meta.Source != SourceBuiltin {
		t.Fatalf("source=%s, want builtin", meta.Source)
	}
	if called {
		t.Fatal("disabled resolve must not hit network")
	}
}

func TestResolve_UsesFreshCacheWithoutNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("fresh cache must short-circuit remote fetch")
	}))
	defer srv.Close()

	to := newTestOpts(t, srv.URL)
	// 预置一份新鲜缓存（fetchedAt=now）
	now := time.Now().UTC().Format(time.RFC3339)
	cache := `{"schemaVersion":1,"fetchedAt":"` + now + `","presets":[` + valid_preset + `]}`
	if err := os.WriteFile(to.opts.CachePath, []byte(cache), 0o600); err != nil {
		t.Fatal(err)
	}

	meta := EnsureResolved(context.Background(), &to.opts)
	// 契约（contracts/api.md §1）：新鲜缓存(<24h)报告为 remote（来自远端且未过期）。
	if meta.Source != SourceRemote {
		t.Fatalf("source=%s, want remote (fresh cache)", meta.Source)
	}
	// 缓存 bundle 必须真正生效（H1：曾缺失 reloadStore）。
	if _, ok := ByID("cached-marker"); !ok {
		t.Fatal("fresh cache preset should be live in store")
	}
}

func TestResolve_StaleCacheStillBeatsFailedFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	to := newTestOpts(t, srv.URL)
	stale := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	cache := `{"schemaVersion":1,"fetchedAt":"` + stale + `","presets":[` + valid_preset + `]}`
	if err := os.WriteFile(to.opts.CachePath, []byte(cache), 0o600); err != nil {
		t.Fatal(err)
	}

	meta := EnsureResolved(context.Background(), &to.opts)
	if meta.Source != SourceCache {
		t.Fatalf("source=%s, want cache (stale but beats failed fetch)", meta.Source)
	}
	if _, ok := ByID("cached-marker"); !ok {
		t.Fatal("stale cache preset should be live in store")
	}
}

func TestRefresh_ForcesFetchAndReportsWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer srv.Close()

	to := newTestOpts(t, srv.URL)
	meta, warning := Refresh(context.Background(), &to.opts)
	if warning != "fetch_failed" {
		t.Fatalf("warning=%q, want fetch_failed", warning)
	}
	if meta.Source != SourceBuiltin {
		t.Fatalf("meta source=%s, want builtin", meta.Source)
	}

	// disabled → 不发请求
	to.opts.Disabled = true
	_, warning2 := Refresh(context.Background(), &to.opts)
	if warning2 != "disabled" {
		t.Fatalf("warning=%q, want disabled", warning2)
	}
}

func TestResolve_RejectsCrossHostRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("cross-host redirect target must not be fetched")
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer srv.Close()

	to := newTestOpts(t, srv.URL)
	meta := EnsureResolved(context.Background(), &to.opts)
	if meta.Source != SourceBuiltin {
		t.Fatalf("source=%s, want builtin (redirect rejected)", meta.Source)
	}
}

func TestSetRemoteDisabled_ImmediateEffect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(remoteBundleJSON(t))
	}))
	defer srv.Close()

	to := newTestOpts(t, srv.URL)
	// 先解析为远端
	if meta := EnsureResolved(context.Background(), &to.opts); meta.Source != SourceRemote {
		t.Fatalf("precondition: source=%s", meta.Source)
	}
	if _, ok := ByID("remote-marker"); !ok {
		t.Fatal("precondition: remote preset live")
	}

	// 关闭：立即回落 builtin 且 meta 如实
	SetRemoteDisabled(true)
	if meta := Meta(); meta.Source != SourceBuiltin {
		t.Fatalf("after disable meta=%s, want builtin", meta.Source)
	}
	if _, ok := ByID("remote-marker"); ok {
		t.Fatal("remote preset must leave store after disable")
	}

	// 再开启后刷新可恢复远端
	SetRemoteDisabled(false)
	meta, warning := Refresh(context.Background(), &to.opts)
	if warning != "" || meta.Source != SourceRemote {
		t.Fatalf("re-enable refresh = %s %q", meta.Source, warning)
	}
}

func TestRefresh_FailureReportsCurrentSourceNotBuiltin(t *testing.T) {
	// 契约 §2.1：已生效 remote 后刷新失败 → meta 仍报 remote（当前生效源）。
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(remoteBundleJSON(t))
	}))
	defer good.Close()
	to := newTestOpts(t, good.URL)
	if meta := EnsureResolved(context.Background(), &to.opts); meta.Source != SourceRemote {
		t.Fatalf("precondition: %s", meta.Source)
	}

	to.opts.RemoteURL = "http://127.0.0.1:1/presets.json"
	meta, warning := Refresh(context.Background(), &to.opts)
	if warning != "fetch_failed" {
		t.Fatalf("warning=%q", warning)
	}
	if meta.Source != SourceRemote {
		t.Fatalf("failure meta=%s, want remote (current effective)", meta.Source)
	}
}
