package probe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func envFor(baseURL, token, model string) map[string]string {
	return map[string]string{
		"ANTHROPIC_BASE_URL":   baseURL,
		"ANTHROPIC_AUTH_TOKEN": token,
		"ANTHROPIC_MODEL":      model,
	}
}

func TestProbe_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path=%s, want /v1/messages", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("Authorization=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","model":"glm-5.3"}`))
	}))
	defer srv.Close()

	res := Probe(context.Background(), envFor(srv.URL, "sk-test", "glm-5.3"), nil)
	if !res.OK {
		t.Fatalf("want ok, got %+v", res.Error)
	}
	if res.Model != "glm-5.3" {
		t.Errorf("model=%q, want server echo", res.Model)
	}
	if res.LatencyMs < 0 {
		t.Errorf("latency=%d", res.LatencyMs)
	}
}

func TestProbe_APITakesPrecedenceOverBearer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "sk-ak" {
			t.Errorf("x-api-key=%q", got)
		}
		_, _ = w.Write([]byte(`{"model":"m"}`))
	}))
	defer srv.Close()

	env := map[string]string{
		"ANTHROPIC_BASE_URL": srv.URL,
		"ANTHROPIC_API_KEY":  "sk-ak",
		"ANTHROPIC_MODEL":    "m",
	}
	res := Probe(context.Background(), env, nil)
	if !res.OK {
		t.Fatalf("want ok, got %+v", res.Error)
	}
}

func TestProbe_Classifications(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{"auth", 401, `{"error":{"message":"invalid x-api-key"}}`, "auth"},
		{"auth403", 403, `forbidden`, "auth"},
		{"model404", 404, `{"error":{"message":"model not found: nope"}}`, "model"},
		{"server500", 500, `boom`, "server"},
		{"unknown400", 400, `bad request`, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			res := Probe(context.Background(), envFor(srv.URL, "sk", "m"), nil)
			if res.OK || res.Error == nil {
				t.Fatalf("want error kind %s, got ok", tc.wantErr)
			}
			if string(res.Error.Kind) != tc.wantErr {
				t.Fatalf("kind=%s, want %s (msg=%s)", res.Error.Kind, tc.wantErr, res.Error.Message)
			}
		})
	}
}

func TestProbe_NetworkError(t *testing.T) {
	// 127.0.0.1:1 立刻连接失败
	res := Probe(context.Background(), envFor("http://127.0.0.1:1", "sk", "m"), nil)
	if res.OK || res.Error.Kind != "network" {
		t.Fatalf("want network error, got %+v", res)
	}
}

func TestProbe_TimeoutClassifiedAsNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	res := Probe(context.Background(), envFor(srv.URL, "sk", "m"), &Options{Timeout: 50 * time.Millisecond})
	if res.OK || res.Error.Kind != "network" {
		t.Fatalf("want network (timeout), got %+v", res)
	}
}

func TestProbe_MissingRequirements(t *testing.T) {
	// 无认证字段
	if res := Probe(context.Background(), map[string]string{"ANTHROPIC_BASE_URL": "https://x"}, nil); res.Error == nil || res.Error.Kind != "invalid" {
		t.Fatalf("want invalid (missing auth), got %+v", res)
	}
	// 自定义 base 但无模型
	env := map[string]string{"ANTHROPIC_BASE_URL": "https://x", "ANTHROPIC_AUTH_TOKEN": "sk"}
	if res := Probe(context.Background(), env, nil); res.Error == nil || res.Error.Kind != "invalid" {
		t.Fatalf("want invalid (missing model), got %+v", res)
	}
}

func TestProbe_OfficialDefaultModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != OfficialDefaultModel {
			t.Errorf("model=%v, want official default", body["model"])
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	// 官方形态：env 显式指向测试服务（等价官方：无自定义模型时的兜底逻辑）
	env := map[string]string{"ANTHROPIC_AUTH_TOKEN": "sk"}
	// OfficialBaseURL 常量不可注入 → 直接验证 defaultModelFor
	if got := defaultModelFor(""); got != OfficialDefaultModel {
		t.Errorf("defaultModelFor('')=%q, want %q", got, OfficialDefaultModel)
	}
	_ = srv
	_ = env
}

func TestProbe_ErrorMessagesStripToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`invalid token sk-secret-abc`))
	}))
	defer srv.Close()

	res := Probe(context.Background(), envFor(srv.URL, "sk-secret-abc", "m"), nil)
	if res.OK {
		t.Fatal("want error")
	}
	if contains(res.Error.Message, "sk-secret-abc") {
		t.Fatalf("error message leaks token: %s", res.Error.Message)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
