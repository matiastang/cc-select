// forward.go 是转发内核（specs/001 contracts/router-http.md §2）：
//
// 每笔已鉴权请求按「当前路由表条目 → provider env（keychain 占位解析）」转发：
//   - 剥离入站 Authorization（伪 token），注入真实凭证——
//     ANTHROPIC_AUTH_TOKEN → `Authorization: Bearer`；ANTHROPIC_API_KEY → `x-api-key`；
//   - 路径/查询/请求体原样透传；
//   - SSE 流式响应即时 flush（FlushInterval 立即）；
//   - 上游非 2xx 原样透传（保留 provider 错误语义）；连接失败 502 + 可读消息。
package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/cc-select/cc-select/internal/config"
)

// EnvResolver 解析 provider env 值（$keychain: 占位 → 真值；T012 的 resolve.go 实现带内存缓存）。
type EnvResolver interface {
	Resolve(value string) (string, error)
}

// identityResolver 原样返回值（无 keychain 场景/测试用）。
type identityResolver struct{}

func (identityResolver) Resolve(v string) (string, error) { return v, nil }

// NewForward 构造转发层。resolver 为 nil 时用恒等解析（明文 env 直接可用）。
func NewForward(resolver EnvResolver) http.Handler {
	if resolver == nil {
		resolver = identityResolver{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entry := entryFrom(r.Context())
		if entry.Provider == "" {
			gatewayError(w, "no route entry on request (auth middleware missing?)")
			return
		}

		rawEnv, err := providerEnvFor(entry.Provider)
		if err != nil {
			gatewayError(w, fmt.Sprintf("route points to provider %q which no longer exists; re-run `ccs use` or `cc-select route switch`", entry.Provider))
			return
		}

		// 解析 env（占位 → 真值）。
		env := make(map[string]string, len(rawEnv))
		for k, v := range rawEnv {
			rv, rerr := resolver.Resolve(v)
			if rerr != nil {
				gatewayError(w, fmt.Sprintf("resolve %s for provider %q: %v", k, entry.Provider, rerr))
				return
			}
			env[k] = rv
		}

		base := env["ANTHROPIC_BASE_URL"]
		if base == "" {
			gatewayError(w, fmt.Sprintf("provider %q has no ANTHROPIC_BASE_URL", entry.Provider))
			return
		}
		upstream, err := url.Parse(base)
		if err != nil {
			gatewayError(w, fmt.Sprintf("provider %q ANTHROPIC_BASE_URL invalid: %v", entry.Provider, err))
			return
		}

		proxy := &httputil.ReverseProxy{
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.SetURL(upstream)
				pr.Out.Host = upstream.Host
				// 剥离伪 token，注入真实凭证（两者可并存，随 provider 配置）。
				pr.Out.Header.Del("Authorization")
				if tok := env[config.AuthTokenVar]; tok != "" {
					pr.Out.Header.Set("Authorization", "Bearer "+tok)
				}
				if key := env["ANTHROPIC_API_KEY"]; key != "" {
					pr.Out.Header.Set("x-api-key", key)
				}
			},
			// SSE 立即 flush：流式输出不被缓冲（SC-006）。
			FlushInterval: -1,
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				gatewayError(w, fmt.Sprintf("upstream connect failed (%s): %v", upstream.Host, err))
			},
		}
		proxy.ServeHTTP(w, r)
	})
}

// gatewayError 输出 502 + 可读错误（不泄密：不回显任何 env 值）。
func gatewayError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"type": "cc_select_gateway", "message": msg},
	})
}
