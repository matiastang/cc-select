package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/profile"
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

// legacy 形态：ANTHROPIC_MODEL 只在 profile settings.json——改写也要能取到。
func TestModelRewrite_LegacyModelFromProfile(t *testing.T) {
	setTempProviders(t, `{"providers":{"MiniMax":{"id":"MiniMax","env":{}}}}`)
	profile.Ensure("MiniMax", map[string]string{"ANTHROPIC_MODEL": "minimax-m3"})
	next, cap := captureNext()

	req := withEntryToReq(
		httptest.NewRequest(http.MethodPost, "http://router.local/v1/messages",
			strings.NewReader(`{"model":"claude-opus-5","x":1}`)),
		routes.Entry{Provider: "MiniMax"})
	req.Header.Set("Content-Type", "application/json")
	ModelRewrite(next).ServeHTTP(httptest.NewRecorder(), req)
	if !strings.Contains(cap.body, `"model":"minimax-m3"`) {
		t.Errorf("legacy provider 的 model 应从 profile 取得并改写: %s", cap.body)
	}
}

// T010：映射化改写契约表全量（specs/002 contracts/proxy-model-routing.md R1~R6）。
// 槽位数据：main=glm-5.3[1m]、opus=glm-5.2[1m]、haiku=glm-5.3-Flash、sonnet 未配置。
func TestModelRewrite_MappingContract(t *testing.T) {
	const glmEnv = `"env":{"ANTHROPIC_MODEL":"glm-5.3[1m]","ANTHROPIC_DEFAULT_OPUS_MODEL":"glm-5.2[1m]","ANTHROPIC_DEFAULT_HAIKU_MODEL":"glm-5.3-Flash"}`

	cases := []struct {
		name     string
		env      string // providers.json 里 glm 的 env JSON 片段
		reqModel string
		want     string // 期望下游收到的 model（空串 = 期望原样透传）
	}{
		// R1：清单内精确透传（含 [1m] 形态）。
		{"R1 清单内 main 透传", glmEnv, "glm-5.3[1m]", ""},
		{"R1 清单内 haiku 透传", glmEnv, "glm-5.3-Flash", ""},
		// R2：opus 家族 → Opus 槽。
		{"R2 完整 id claude-opus-5", glmEnv, "claude-opus-5", "glm-5.2[1m]"},
		{"R2 别名 opus", glmEnv, "opus", "glm-5.2[1m]"},
		{"R2 别名 opus[1m]", glmEnv, "opus[1m]", "glm-5.2[1m]"},
		// R3：sonnet 家族 → sonnet 槽空 → 回落 main。
		{"R3 sonnet 槽空回落 main", glmEnv, "claude-sonnet-5", "glm-5.3[1m]"},
		{"R3 别名 sonnet", glmEnv, "sonnet", "glm-5.3[1m]"},
		// R4：haiku 家族 → Haiku 槽（后台小任务路径）。
		{"R4 别名 haiku", glmEnv, "haiku", "glm-5.3-Flash"},
		{"R4 完整 id claude-haiku-4-5", glmEnv, "claude-haiku-4-5-20251001", "glm-5.3-Flash"},
		// R5：未知 id（热切后旧 provider 的模型）→ 主模型（v1 兼容）。
		{"R5 未知 id 回落 main", glmEnv, "glm-4.6", "glm-5.3[1m]"},
		{"R5 default 别名", glmEnv, "default", "glm-5.3[1m]"},
		// R6：未配置 ANTHROPIC_MODEL → 全路径透传（v1 回归）。
		{"R6 无 main 透传", `"env":{}`, "claude-opus-5", ""},
		{"R6 无 main 未知 id 也透传", `"env":{}`, "whatever", ""},
		// 清单精确匹配优先于子串分类：provider id 恰含 "sonnet" 不得误判。
		{"R1 优先于子串：id 含 sonnet", `"env":{"ANTHROPIC_MODEL":"acme-sonnet-x"}`, "acme-sonnet-x", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setTempProviders(t, `{"providers":{"glm":{"id":"glm",`+c.env+`}}}`)
			next, cap := captureNext()
			body := `{"model":"` + c.reqModel + `","x":1}`
			ModelRewrite(next).ServeHTTP(httptest.NewRecorder(), rewriteReq(t, body, "application/json"))
			if c.want == "" {
				if cap.body != body {
					t.Errorf("应逐字节透传: got %s", cap.body)
				}
				return
			}
			if !strings.Contains(cap.body, `"model":"`+c.want+`"`) {
				t.Errorf("model 应改写为 %s: %s", c.want, cap.body)
			}
			if strings.Contains(cap.body, c.reqModel) {
				t.Errorf("旧 model 不得残留: %s", cap.body)
			}
		})
	}
}
