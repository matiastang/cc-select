package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/routes"
)

// captureNext 记录下游收到的 body 与 Content-Length。
func captureNext() (http.Handler, *captured) {
	c := &captured{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		c.body = string(b)
		c.length = r.ContentLength
		c.lengthHeader = r.Header.Get("Content-Length")
		w.WriteHeader(http.StatusOK)
	}), c
}

type captured struct {
	body         string
	length       int64
	lengthHeader string
}

func rewriteReq(t *testing.T, body, contentType string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://router.local/v1/messages", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return withEntryToReq(req, routes.Entry{Provider: "glm"})
}

func TestModelRewrite_ReplacesModelAndFixesLength(t *testing.T) {
	setTempProviders(t, `{"providers":{"glm":{"id":"glm","env":{"ANTHROPIC_MODEL":"glm-5.1"}}}}`)
	next, cap := captureNext()

	body := `{"model":"claude-sonnet-5","max_tokens":1024,"messages":[{"role":"user","content":"hi"}]}`
	ModelRewrite(next).ServeHTTP(httptest.NewRecorder(), rewriteReq(t, body, "application/json"))

	if !strings.Contains(cap.body, `"model":"glm-5.1"`) {
		t.Errorf("model 应改写: %s", cap.body)
	}
	if strings.Contains(cap.body, "claude-sonnet-5") {
		t.Errorf("旧 model 不得残留: %s", cap.body)
	}
	if !strings.Contains(cap.body, `"max_tokens":1024`) {
		t.Errorf("其余字段应保留（数字不被精度改写）: %s", cap.body)
	}
	if cap.length != int64(len(cap.body)) || cap.lengthHeader != strconv.Itoa(len(cap.body)) {
		t.Errorf("Content-Length 应更新: len=%d header=%q bodylen=%d", cap.length, cap.lengthHeader, len(cap.body))
	}
}

func TestModelRewrite_NoModelConfig_Passthrough(t *testing.T) {
	setTempProviders(t, `{"providers":{"glm":{"id":"glm","env":{}}}}`)
	next, cap := captureNext()

	body := `{"model":"claude-sonnet-5","x":1}`
	ModelRewrite(next).ServeHTTP(httptest.NewRecorder(), rewriteReq(t, body, "application/json"))
	if cap.body != body {
		t.Errorf("未配 ANTHROPIC_MODEL 应逐字节透传: got %s", cap.body)
	}
}

func TestModelRewrite_NonJSONPassthrough(t *testing.T) {
	setTempProviders(t, `{"providers":{"glm":{"id":"glm","env":{"ANTHROPIC_MODEL":"glm-5.1"}}}}`)
	next, cap := captureNext()

	body := "plain-binary-or-form-payload"
	ModelRewrite(next).ServeHTTP(httptest.NewRecorder(), rewriteReq(t, body, "application/octet-stream"))
	if cap.body != body {
		t.Errorf("非 JSON 应透传: got %s", cap.body)
	}
}

func TestModelRewrite_NoBodyPassthrough(t *testing.T) {
	setTempProviders(t, `{"providers":{"glm":{"id":"glm","env":{"ANTHROPIC_MODEL":"glm-5.1"}}}}`)
	next, cap := captureNext()

	req := withEntryToReq(httptest.NewRequest(http.MethodGet, "http://router.local/health", nil), routes.Entry{Provider: "glm"})
	ModelRewrite(next).ServeHTTP(httptest.NewRecorder(), req)
	if cap.body != "" {
		t.Errorf("无 body 应透传: got %q", cap.body)
	}
}

func TestModelRewrite_InvalidJSONPassthrough(t *testing.T) {
	setTempProviders(t, `{"providers":{"glm":{"id":"glm","env":{"ANTHROPIC_MODEL":"glm-5.1"}}}}`)
	next, cap := captureNext()

	body := `{not-json`
	ModelRewrite(next).ServeHTTP(httptest.NewRecorder(), rewriteReq(t, body, "application/json"))
	if cap.body != body {
		t.Errorf("非法 JSON 应透传不改（不破坏在途内容）: got %s", cap.body)
	}
}

func TestModelRewrite_NoModelFieldPassthrough(t *testing.T) {
	setTempProviders(t, `{"providers":{"glm":{"id":"glm","env":{"ANTHROPIC_MODEL":"glm-5.1"}}}}`)
	next, cap := captureNext()

	body := `{"messages":[]}`
	ModelRewrite(next).ServeHTTP(httptest.NewRecorder(), rewriteReq(t, body, "application/json"))
	if cap.body != body {
		t.Errorf("无 model 字段应透传: got %s", cap.body)
	}
}
