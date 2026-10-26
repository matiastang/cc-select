package usage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cc-select/cc-select/internal/probe"
)

func optsFor(srvURL string) *probe.Options {
	return nil // 生产默认；探测目标由 env base URL 指向 httptest
}

func TestRegistry_MatchesByHost(t *testing.T) {
	cases := []struct {
		baseURL string
		want    string
	}{
		{"https://open.bigmodel.cn/api/anthropic", "glm"},
		{"https://api.z.ai/api/anthropic", "glm"},
		{"https://api.moonshot.cn/anthropic", "kimi-balance"},
		{"https://api.deepseek.com/anthropic", "anthropic-headers"},
		{"", "anthropic-headers"}, // 官方形态 → 头探测 api.anthropic.com
	}
	for _, tc := range cases {
		a := matchAdapter(tc.baseURL)
		if a == nil || a.ID() != tc.want {
			t.Fatalf("match(%q)=%v, want %s", tc.baseURL, a, tc.want)
		}
	}
}

func TestAnthropicHeaders_ParsesUnifiedWindows(t *testing.T) {
	reset := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC1123)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// 形态一：独立 unified 头（5h/week 窗口）
		w.Header().Set("anthropic-ratelimit-unified-5h-limit", "100")
		w.Header().Set("anthropic-ratelimit-unified-5h-remaining", "55")
		w.Header().Set("anthropic-ratelimit-unified-5h-reset", reset)
		w.Header().Set("anthropic-ratelimit-unified-week-limit", "2400")
		w.Header().Set("anthropic-ratelimit-unified-week-remaining", "1800")
		_, _ = w.Write([]byte(`{"model":"glm-5.3"}`))
	}))
	defer srv.Close()

	rep, err := Query(context.Background(), envFor(srv.URL, "tok"))
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Supported || len(rep.Windows) < 2 {
		t.Fatalf("want 2 windows, got supported=%v %+v", rep.Supported, rep.Windows)
	}
	var five *Window
	for i, w := range rep.Windows {
		if w.Type == "5h" {
			five = &rep.Windows[i]
		}
	}
	if five == nil || five.Limit == nil || *five.Limit != 100 || five.Used == nil || *five.Used != 45 {
		t.Fatalf("5h window = %+v", five)
	}
	if five.ResetAt == nil {
		t.Fatal("5h resetAt should parse")
	}
}

func TestAnthropicHeaders_ParsesMinuteHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("anthropic-ratelimit-requests-limit", "50")
		w.Header().Set("anthropic-ratelimit-requests-remaining", "49")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	rep, err := Query(context.Background(), envFor(srv.URL, "tok"))
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Supported {
		t.Fatalf("minute headers should count, reason=%s", rep.Reason)
	}
	found := false
	for _, w := range rep.Windows {
		if w.Type == "minute-requests" {
			found = true
		}
	}
	if !found {
		t.Fatalf("minute-requests window missing: %+v", rep.Windows)
	}
}

func TestAnthropicHeaders_UnsupportedWhenNoHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	rep, err := Query(context.Background(), envFor(srv.URL, "tok"))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Supported || rep.Reason != "no_limit_headers" {
		t.Fatalf("want unsupported/no_limit_headers, got %+v", rep)
	}
}

func TestAnthropicHeaders_QueryErrorSurfaces(t *testing.T) {
	_, err := Query(context.Background(), envFor("http://127.0.0.1:1", "tok"))
	if err == nil {
		t.Fatal("network error should surface")
	}
	if strings.Contains(err.Error(), "tok") {
		t.Fatal("error must not leak token")
	}
}

func envFor(baseURL, token string) map[string]string {
	return map[string]string{
		"ANTHROPIC_BASE_URL":   baseURL,
		"ANTHROPIC_AUTH_TOKEN": token,
		"ANTHROPIC_MODEL":      "m",
	}
}

func TestGLMQuota_ParsesLimits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/monitor/usage/quota/limit" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "tok-glm" {
			t.Errorf("Authorization=%q (no Bearer prefix)", got)
		}
		_, _ = w.Write([]byte(`{"data":{"limits":[
			{"type":"TOKENS_LIMIT","percentage":63.5},
			{"type":"TIME_LIMIT","percentage":20}
		]}}`))
	}))
	defer srv.Close()

	rep, err := (glmQuota{}).Query(context.Background(), envFor(srv.URL, "tok-glm"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Supported || len(rep.Windows) != 2 {
		t.Fatalf("want 2 windows, got %+v", rep)
	}
	if rep.Windows[0].Type != Window5h || rep.Windows[0].Percent == nil || *rep.Windows[0].Percent != 63.5 {
		t.Fatalf("5h window = %+v", rep.Windows[0])
	}
	if rep.Windows[1].Type != WindowMonthMCP {
		t.Fatalf("month-mcp window = %+v", rep.Windows[1])
	}
}

func TestKimiBalance_ParsesBalance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/users/me/balance" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok-kimi" {
			t.Errorf("Authorization=%q", got)
		}
		_, _ = w.Write([]byte(`{"available_balance":7.5,"balance":10}`))
	}))
	defer srv.Close()

	// moonshot host 匹配但 URL 指向本地——用 hostOf 不可注入；改为直接实例化适配器
	env := map[string]string{
		"ANTHROPIC_BASE_URL":   srv.URL,
		"ANTHROPIC_AUTH_TOKEN": "tok-kimi",
	}
	rep, err := (kimiBalance{}).Query(context.Background(), env, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Supported || len(rep.Windows) != 1 {
		t.Fatalf("want balance window, got %+v", rep)
	}
	w0 := rep.Windows[0]
	if w0.Type != WindowBalance || w0.Used == nil || *w0.Used != 2.5 || w0.Limit == nil || *w0.Limit != 10 {
		t.Fatalf("balance window = %+v", w0)
	}
}

func TestGLMQuota_EmptyLimitsReportsUnsupported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"limits":[]}}`))
	}))
	defer srv.Close()

	env := map[string]string{
		"ANTHROPIC_BASE_URL":   srv.URL,
		"ANTHROPIC_AUTH_TOKEN": "tok-glm",
	}
	rep, err := (glmQuota{}).Query(context.Background(), env, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Supported || rep.Reason != "no_limit_data" {
		t.Fatalf("want unsupported/no_limit_data, got %+v", rep)
	}
}
