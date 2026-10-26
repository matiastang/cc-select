package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestProbe_SanitizeAcrossTruncateBoundary(t *testing.T) {
	// token 跨第 200 rune 边界：先脱敏再截断，任何残段不得泄露。
	long := strings.Repeat("x", 195)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(long + "sk-boundary-token-abcdefghijklmnop"))
	}))
	defer srv.Close()

	res := Probe(context.Background(), envFor(srv.URL, "sk-boundary-token-abcdefghijklmnop", "m"), nil)
	if res.OK {
		t.Fatal("want error")
	}
	if strings.Contains(res.Error.Message, "sk-boundary") || strings.Contains(res.Error.Message, "boundary-token") {
		t.Fatalf("token fragment leaked: %q", res.Error.Message)
	}
}

func TestProbe_OfficialFormUsesDefaultModel(t *testing.T) {
	// 官方形态（无 base URL）：经注入 Transport 定向到本地服务，验证默认模型与 /v1/messages 路径。
	var gotModel, gotPath string
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		if r.URL.Host != "api.anthropic.com" {
			return nil, fmt.Errorf("unexpected host: %s", r.URL.Host)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.NewDecoder(bytes.NewReader(body)).Decode(&payload)
		gotModel, _ = payload["model"].(string)
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"model":"echo"}`)),
			Request:    r,
		}, nil
	})
	env := map[string]string{"ANTHROPIC_AUTH_TOKEN": "sk-official"}
	res := Probe(context.Background(), env, &Options{Client: &http.Client{Transport: rt}})
	if !res.OK || res.Model != "echo" {
		t.Fatalf("official probe = %+v", res)
	}
	if gotPath != "/v1/messages" {
		t.Errorf("path=%s", gotPath)
	}
	if gotModel != OfficialDefaultModel {
		t.Errorf("default model=%q, want %q", gotModel, OfficialDefaultModel)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
