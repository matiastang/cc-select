package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cc-select/cc-select/internal/profile"
	"github.com/cc-select/cc-select/internal/routes"
)

// fakeResolver 是注入的 env 解析器（$keychain: 占位 → 真值）。
type fakeResolver map[string]string

func (f fakeResolver) Resolve(v string) (string, error) {
	if s, ok := f[v]; ok {
		return s, nil
	}
	return v, nil
}

// upstreamRecorder 记录上游收到的请求，可编程响应。
type upstreamRecorder struct {
	URL         string
	authz       []string
	apiKeys     []string
	bodies      []string
	paths       []string
	respStatus  int
	respBody    string
	handlerFunc http.HandlerFunc
}

func newUpstream(t *testing.T, rec *upstreamRecorder) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rec.authz = append(rec.authz, r.Header.Get("Authorization"))
		rec.apiKeys = append(rec.apiKeys, r.Header.Get("x-api-key"))
		rec.bodies = append(rec.bodies, string(b))
		rec.paths = append(rec.paths, r.URL.Path)
		if rec.handlerFunc != nil {
			rec.handlerFunc(w, r)
			return
		}
		w.WriteHeader(rec.respStatus)
		_, _ = w.Write([]byte(rec.respBody))
	})
	srv := httptest.NewServer(mux)
	rec.URL = srv.URL
	t.Cleanup(srv.Close)
	return srv
}

// setTempProviders 写入一份临时 providers.json（CC_SELECT_CONFIG 指向它）。
func setTempProviders(t *testing.T, json string) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "providers.json")
	if err := os.WriteFile(p, []byte(json), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CC_SELECT_CONFIG", p)
}

// withEntryToReq（定义见上）。

func TestForward_InjectsBearerAndStripsPseudoToken(t *testing.T) {
	rec := &upstreamRecorder{respStatus: 200, respBody: `{"ok":true}`}
	srv := newUpstream(t, rec)
	setTempProviders(t, `{"providers":{"glm":{"id":"glm","env":{"ANTHROPIC_BASE_URL":"`+srv.URL+`","ANTHROPIC_AUTH_TOKEN":"sk-real"}}}}`)

	fwd := NewForward(nil)
	entry := routes.Entry{TID: "ccs-11111111111111111111111111111111", Provider: "glm"}
	req := httptest.NewRequest(http.MethodPost, "http://router.local/v1/messages", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer ccs-11111111111111111111111111111111")
	w := httptest.NewRecorder()
	fwd.ServeHTTP(w, withEntryToReq(req, entry))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
	}
	if rec.authz[0] != "Bearer sk-real" {
		t.Errorf("上游应收到真 token，got %q", rec.authz[0])
	}
	if strings.Contains(rec.authz[0], "ccs-") {
		t.Errorf("伪 token 不得透传上游: %q", rec.authz[0])
	}
	if rec.paths[0] != "/v1/messages" {
		t.Errorf("路径应透传: %q", rec.paths[0])
	}
}

// withEntryToReq 把路由条目挂进请求上下文（模拟 auth 中间件产物）。
func withEntryToReq(req *http.Request, e routes.Entry) *http.Request {
	return req.WithContext(withEntry(req.Context(), e))
}

func TestForward_APIKeyHeader(t *testing.T) {
	rec := &upstreamRecorder{respStatus: 200, respBody: `{}`}
	srv := newUpstream(t, rec)
	setTempProviders(t, `{"providers":{"glm":{"id":"glm","env":{"ANTHROPIC_BASE_URL":"`+srv.URL+`","ANTHROPIC_API_KEY":"sk-ak"}}}}`)

	fwd := NewForward(nil)
	req := httptest.NewRequest(http.MethodPost, "http://router.local/v1/messages", nil)
	w := httptest.NewRecorder()
	fwd.ServeHTTP(w, withEntryToReq(req, routes.Entry{Provider: "glm"}))

	if rec.apiKeys[0] != "sk-ak" {
		t.Errorf("ANTHROPIC_API_KEY 应映射为 x-api-key: got %q", rec.apiKeys[0])
	}
	if rec.authz[0] != "" {
		t.Errorf("未配 AUTH_TOKEN 时不应有 Bearer: got %q", rec.authz[0])
	}
}

func TestForward_ResolvesKeychainPlaceholder(t *testing.T) {
	rec := &upstreamRecorder{respStatus: 200, respBody: `{}`}
	srv := newUpstream(t, rec)
	setTempProviders(t, `{"providers":{"glm":{"id":"glm","env":{"ANTHROPIC_BASE_URL":"`+srv.URL+`","ANTHROPIC_AUTH_TOKEN":"$keychain:cc-select:glm:ANTHROPIC_AUTH_TOKEN"}}}}`)

	fwd := NewForward(fakeResolver{"$keychain:cc-select:glm:ANTHROPIC_AUTH_TOKEN": "sk-from-keychain"})
	req := httptest.NewRequest(http.MethodPost, "http://router.local/v1/messages", nil)
	w := httptest.NewRecorder()
	fwd.ServeHTTP(w, withEntryToReq(req, routes.Entry{Provider: "glm"}))

	if rec.authz[0] != "Bearer sk-from-keychain" {
		t.Errorf("占位应解析为 keychain 真值: got %q", rec.authz[0])
	}
}

func TestForward_UpstreamErrorPassthrough(t *testing.T) {
	rec := &upstreamRecorder{respStatus: 429, respBody: `{"error":"rate_limited"}`}
	srv := newUpstream(t, rec)
	setTempProviders(t, `{"providers":{"glm":{"id":"glm","env":{"ANTHROPIC_BASE_URL":"`+srv.URL+`"}}}}`)

	fwd := NewForward(nil)
	req := httptest.NewRequest(http.MethodPost, "http://router.local/v1/messages", nil)
	w := httptest.NewRecorder()
	fwd.ServeHTTP(w, withEntryToReq(req, routes.Entry{Provider: "glm"}))

	if w.Code != 429 || !strings.Contains(w.Body.String(), "rate_limited") {
		t.Errorf("上游错误应原样透传: code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestForward_UpstreamUnreachableReturns502(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	url := dead.URL
	dead.Close()
	setTempProviders(t, `{"providers":{"glm":{"id":"glm","env":{"ANTHROPIC_BASE_URL":"`+url+`"}}}}`)

	fwd := NewForward(nil)
	req := httptest.NewRequest(http.MethodPost, "http://router.local/v1/messages", nil)
	w := httptest.NewRecorder()
	fwd.ServeHTTP(w, withEntryToReq(req, routes.Entry{Provider: "glm"}))

	if w.Code != http.StatusBadGateway {
		t.Fatalf("上游不可达 want 502 got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "upstream") && !strings.Contains(w.Body.String(), "上游") {
		t.Errorf("502 体应含可读错误: %s", w.Body.String())
	}
}

func TestForward_RouteProviderMissing(t *testing.T) {
	setTempProviders(t, `{"providers":{}}`) // 路由表指向的 glm 已被删除
	fwd := NewForward(nil)
	req := httptest.NewRequest(http.MethodPost, "http://router.local/v1/messages", nil)
	w := httptest.NewRecorder()
	fwd.ServeHTTP(w, withEntryToReq(req, routes.Entry{Provider: "glm"}))
	if w.Code != http.StatusBadGateway {
		t.Errorf("路由指向不存在的 provider want 502 got %d", w.Code)
	}
}

func TestForward_SSEFlushedImmediately(t *testing.T) {
	released := make(chan time.Time, 1)
	rec := &upstreamRecorder{}
	rec.handlerFunc = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: chunk1\n\n"))
		w.(http.Flusher).Flush()
		<-released // 保持连接不结束
		_, _ = w.Write([]byte("data: chunk2\n\n"))
	}
	srv := newUpstream(t, rec)
	setTempProviders(t, `{"providers":{"glm":{"id":"glm","env":{"ANTHROPIC_BASE_URL":"`+srv.URL+`"}}}}`)

	fwd := NewForward(nil)
	// 真实 HTTP 客户端读流（httptest.ResponseRecorder 无法模拟流式时序）。
	clientSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fwd.ServeHTTP(w, withEntryToReq(r, routes.Entry{Provider: "glm"}))
	}))
	t.Cleanup(clientSrv.Close)

	type result struct {
		chunk string
		after time.Duration
		err   string
	}
	got := make(chan result, 1)
	go func() {
		resp, err := http.Get(clientSrv.URL + "/v1/messages")
		if err != nil {
			got <- result{err: err.Error()}
			return
		}
		defer resp.Body.Close()
		start := time.Now()
		buf := make([]byte, 64)
		n, _ := resp.Body.Read(buf)
		got <- result{chunk: string(buf[:n]), after: time.Since(start)}
	}()

	select {
	case r := <-got:
		if !strings.Contains(r.chunk, "chunk1") {
			t.Errorf("首块应含 chunk1: %q", r.chunk)
		}
		if r.after > 900*time.Millisecond {
			t.Errorf("SSE 应即时 flush（首块耗时 %v）", r.after)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("2s 内未收到首块——SSE 被缓冲")
	}
	released <- time.Now() // 放行上游结束
}

// legacy 形态：env 真值只在 profile settings.json（providers.json env 为空）。
// daemon 必须回退读 profile，否则 route switch 到一个从未 use 过的 provider 即 502。
func TestForward_LegacyProviderEnvFallsBackToProfile(t *testing.T) {
	rec := &upstreamRecorder{respStatus: 200, respBody: `{}`}
	srv := newUpstream(t, rec)
	// providers.json：MiniMax env 为空。
	setTempProviders(t, `{"providers":{"MiniMax":{"id":"MiniMax","name":"MiniMax"}}}`)
	// profile settings.json：真值在这（用户在 GUI 原文编辑时代配置的）。
	profile.Ensure("MiniMax", map[string]string{
		"ANTHROPIC_BASE_URL":   srv.URL,
		"ANTHROPIC_AUTH_TOKEN": "sk-legacy",
		"ANTHROPIC_MODEL":      "minimax-m3",
	})

	fwd := NewForward(nil)
	req := httptest.NewRequest(http.MethodPost, "http://router.local/v1/messages", nil)
	w := httptest.NewRecorder()
	fwd.ServeHTTP(w, withEntryToReq(req, routes.Entry{Provider: "MiniMax"}))

	if w.Code != http.StatusOK {
		t.Fatalf("legacy provider 应可转发: code=%d body=%s", w.Code, w.Body.String())
	}
	if rec.authz[0] != "Bearer sk-legacy" {
		t.Errorf("应使用 profile 里的真值 token: got %q", rec.authz[0])
	}
}
