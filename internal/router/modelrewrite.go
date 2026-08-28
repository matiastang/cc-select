// modelrewrite.go 在请求进入转发层前改写 JSON body 的 model 字段（研究 D9/L3）。
//
// 背景：会话内切换后，claude 仍按启动时认知发送旧 provider 的 model id；
// 改写必须在代理侧完成。规则：
//   - body 是 JSON object 且含 "model" 字段，且当前路由 provider 的 env 定义了
//     ANTHROPIC_MODEL（且非 keychain 占位——模型名不是敏感值）→ 替换为该值；
//   - 其余一切情况（未配置 / 非 JSON / 非法 JSON / 无 model 字段 / 无 body）
//     → 逐字节透传，不破坏在途内容。
//
// 已知简化（L3）：所有出现的 model 统一映射到主模型（后台小任务也用主模型）。
package router

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/cc-select/cc-select/internal/config"
)

// ModelRewrite 是 model 改写中间件：包在 forward 外层（auth → ModelRewrite → forward）。
func ModelRewrite(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entry := entryFrom(r.Context())
		if entry.Provider == "" || r.Body == nil || r.ContentLength == 0 {
			next.ServeHTTP(w, r)
			return
		}
		if !strings.Contains(r.Header.Get("Content-Type"), "json") {
			next.ServeHTTP(w, r)
			return
		}

		model, ok := targetModel(entry.Provider)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		// 体积上限（评审 #7）：超限的巨型 body 原样透传不改写（防内存放大）。
		const maxRewriteBody = 32 << 20 // 32 MiB——远大于正常 Messages 请求
		body, err := io.ReadAll(io.LimitReader(r.Body, maxRewriteBody+1))
		_ = r.Body.Close()
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		if len(body) > maxRewriteBody {
			passBody(w, r, next, body)
			return
		}

		// 解析失败（含非 object JSON）/ 无 model 字段 → 原样透传。
		var obj map[string]any
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber() // 保留数字字面量，避免精度改写
		if err := dec.Decode(&obj); err != nil {
			passBody(w, r, next, body)
			return
		}
		if _, has := obj["model"]; !has {
			passBody(w, r, next, body)
			return
		}

		obj["model"] = model
		rewritten, err := json.Marshal(obj)
		if err != nil {
			passBody(w, r, next, body)
			return
		}
		passBody(w, r, next, rewritten)
	})
}

// passBody 以给定字节串恢复请求体后放行下游（同步 Content-Length）。
func passBody(w http.ResponseWriter, r *http.Request, next http.Handler, body []byte) {
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.Header.Set("Content-Length", strconv.Itoa(len(body)))
	next.ServeHTTP(w, r)
}

// targetModel 查当前路由 provider 配置的 ANTHROPIC_MODEL；未配置/为占位 → ok=false。
func targetModel(providerID string) (string, bool) {
	env, err := providerEnvFor(providerID)
	if err != nil {
		return "", false
	}
	m := env["ANTHROPIC_MODEL"]
	if m == "" || config.IsKeychainPlaceholder(m) {
		return "", false
	}
	return m, true
}
