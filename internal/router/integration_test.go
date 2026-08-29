//go:build integration

// integration_test.go 是 Mode P 的全链路集成测试（研究 D13）：
// 伪 claude 客户端（Bearer tid）→ 真 daemon（鉴权/改写/转发）→ fake upstream。
// 覆盖：切换后下一笔生效、在途请求按原 provider 完成、model 改写、401 指引。
package router

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cc-select/cc-select/internal/routes"
)

// startDaemon 起一个真实监听的完整 daemon（auth → ModelRewrite → forward）。
func startDaemon(t *testing.T) string {
	t.Helper()
	pipeline := ModelRewrite(NewForward(nil))
	s := NewServer("127.0.0.1:0", "0.0.6", pipeline)
	ln, err := s.Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := ln.Addr().String()
	go func() { _ = s.Serve(ln) }()
	t.Cleanup(func() { _ = s.Close() })
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := Probe(addr); err == nil {
			return addr
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("daemon 未就绪（%s）", addr)
	return ""
}

// setupTwoProviders 起 glm/minimax 两个 fake upstream 并写入 providers.json。
func setupTwoProviders(t *testing.T) (glm, minimax *upstreamRecorder) {
	t.Helper()
	glm = &upstreamRecorder{respStatus: 200, respBody: `{"upstream":"glm"}`}
	minimax = &upstreamRecorder{respStatus: 200, respBody: `{"upstream":"minimax"}`}
	glmSrv := newUpstream(t, glm)
	mmSrv := newUpstream(t, minimax)
	setTempProviders(t, fmt.Sprintf(`{"providers":{`+
		`"glm":{"id":"glm","env":{"ANTHROPIC_BASE_URL":"%s","ANTHROPIC_AUTH_TOKEN":"sk-glm","ANTHROPIC_MODEL":"glm-5.1"}},`+
		`"minimax":{"id":"minimax","env":{"ANTHROPIC_BASE_URL":"%s","ANTHROPIC_AUTH_TOKEN":"sk-mm","ANTHROPIC_MODEL":"MiniMax-M2"}}}}`,
		glmSrv.URL, mmSrv.URL))
	return glm, minimax
}

// claudePost 模拟 claude 的一笔请求（Bearer 伪 token + JSON body）。
func claudePost(t *testing.T, addr, tid, model string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/v1/messages",
		strings.NewReader(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}]}`, model)))
	if err != nil {
		t.Fatalf("构造请求: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+tid)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestIntegration_SwitchTakesEffectNextRequest(t *testing.T) {
	glm, minimax := setupTwoProviders(t)
	addr := startDaemon(t)
	tid, _ := routes.NewTID()
	if err := routes.Set(tid, "glm"); err != nil {
		t.Fatal(err)
	}

	// 第 1 笔：走 glm，model 已按 glm 配置改写，token 为真值。
	code, body := claudePost(t, addr, tid, "claude-sonnet-5")
	if code != 200 || !strings.Contains(body, `"glm"`) {
		t.Fatalf("第 1 笔应走 glm: code=%d body=%s", code, body)
	}
	if glm.authz[0] != "Bearer sk-glm" {
		t.Errorf("glm 上游应收真 token: %q", glm.authz[0])
	}
	if !strings.Contains(glm.bodies[0], `"model":"glm-5.1"`) {
		t.Errorf("model 应改写为 glm-5.1: %s", glm.bodies[0])
	}

	// 会话内切换（= route switch 的全部副作用：改路由表）。
	if err := routes.Set(tid, "minimax"); err != nil {
		t.Fatal(err)
	}

	// 第 2 笔：下一笔即走 minimax，model 换映射，claude 无需重启。
	code, body = claudePost(t, addr, tid, "claude-sonnet-5")
	if code != 200 || !strings.Contains(body, `"minimax"`) {
		t.Fatalf("切换后下一笔应走 minimax: code=%d body=%s", code, body)
	}
	if minimax.authz[0] != "Bearer sk-mm" {
		t.Errorf("minimax 上游应收真 token: %q", minimax.authz[0])
	}
	if !strings.Contains(minimax.bodies[0], `"model":"MiniMax-M2"`) {
		t.Errorf("model 应改写为 MiniMax-M2: %s", minimax.bodies[0])
	}
	if len(glm.bodies) != 1 {
		t.Errorf("glm 不应再收到请求: %d 笔", len(glm.bodies))
	}
}

func TestIntegration_InflightCompletesOnOldProvider(t *testing.T) {
	glm, _ := setupTwoProviders(t)
	addr := startDaemon(t)
	tid, _ := routes.NewTID()
	if err := routes.Set(tid, "glm"); err != nil {
		t.Fatal(err)
	}

	// glm 上游挂起一笔在途请求，到达即通知。
	arrived := make(chan struct{})
	release := make(chan struct{})
	glm.handlerFunc = func(w http.ResponseWriter, r *http.Request) {
		close(arrived)
		<-release
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"upstream":"glm-inflight"}`))
	}
	type resp struct {
		code int
		body string
	}
	done := make(chan resp, 1)
	go func() {
		c, b := claudePost(t, addr, tid, "claude-sonnet-5")
		done <- resp{c, b}
	}()
	select {
	case <-arrived:
	case <-time.After(2 * time.Second):
		t.Fatal("在途请求 2s 内未到达 glm 上游")
	}

	// 在途期间切换路由。
	if err := routes.Set(tid, "minimax"); err != nil {
		t.Fatal(err)
	}
	close(release)

	select {
	case r := <-done:
		if r.code != 200 || !strings.Contains(r.body, "glm-inflight") {
			t.Errorf("在途请求应按原 provider 完成: code=%d body=%s", r.code, r.body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("在途请求未完成")
	}
	// 其后新请求走 minimax。
	if _, body := claudePost(t, addr, tid, "claude-sonnet-5"); !strings.Contains(body, `"minimax"`) {
		t.Errorf("释放后的新请求应走 minimax: %s", body)
	}
}

func TestIntegration_UnknownTIDGets401Guidance(t *testing.T) {
	_, _ = setupTwoProviders(t)
	addr := startDaemon(t)
	stranger := "ccs-99999999999999999999999999999999"

	code, body := claudePost(t, addr, stranger, "claude-sonnet-5")
	if code != http.StatusUnauthorized {
		t.Fatalf("未知 tid want 401 got %d", code)
	}
	if !strings.Contains(body, "ccs use") {
		t.Errorf("401 应含指引: %s", body)
	}
}
