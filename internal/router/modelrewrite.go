// modelrewrite.go 在请求进入转发层前按映射契约改写 JSON body 的 model 字段
// （specs/002 contracts/proxy-model-routing.md——001 的「统一映射主模型」简化
// （L3）按既定进化方向升级为映射化规则）。
//
// 背景：会话内切换后，claude 仍按启动时认知发送旧 provider 的 model id；
// 改写必须在代理侧完成。用户在 /model 选择的模型（∈ provider 清单）原样透传，
// 内置目录/别名按槽位映射，未知 id 回落主模型，未配置主模型则全路径透传。
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

		// 解析失败（含非 object JSON）→ 原样透传。
		var obj map[string]any
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber() // 保留数字字面量，避免精度改写
		if err := dec.Decode(&obj); err != nil {
			passBody(w, r, next, body)
			return
		}
		reqModel, has := obj["model"].(string)
		if !has {
			passBody(w, r, next, body)
			return
		}

		target, rewrite := resolveTargetModel(entry.Provider, reqModel)
		if !rewrite {
			passBody(w, r, next, body)
			return
		}
		obj["model"] = target
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

// resolveTargetModel 按映射契约（specs/002 contracts/proxy-model-routing.md R1~R6）
// 把请求体 model 解析为转发值；ok=false 表示原样透传。
//
// 规则（按序，首个命中）：
//  1. 精确等于 provider 清单某 id（含 [1m] 形态）→ 透传（用户在 /model 的选择）；
//  2. 含子串 opus/sonnet/haiku（不区分大小写）→ 对应槽位变量；槽空 → 回落主模型；
//  3. 其余（含热切后旧 provider 的 id）→ 主模型（v1「无条件主模型」兼容）；
//  4. 主模型为空（未配 ANTHROPIC_MODEL）→ 透传（v1 逐字节兼容）。
//
// 清单精确匹配优先于子串分类：provider id 恰含 "sonnet" 之类子串时不得误判。
// 槽位数据经 config.ModelPlanFromEnv 派生——与 /model 注入清单严格同源。
func resolveTargetModel(providerID, reqModel string) (string, bool) {
	env, err := providerEnvFor(providerID)
	if err != nil {
		return "", false
	}
	plan := config.ModelPlanFromEnv(env)

	// R1：清单内 → 透传。
	for _, e := range plan.Entries {
		if e.ID == reqModel {
			return "", false
		}
	}

	// R6 前置：无任何模型变量 → 全路径透传。
	fallback := plan.Main
	if fallback == "" && len(plan.Entries) == 0 {
		return "", false
	}

	// R2~R4：内置目录/别名形态 → 槽位映射。
	lm := strings.ToLower(reqModel)
	target := ""
	switch {
	case strings.Contains(lm, "opus"):
		target = plan.Opus
	case strings.Contains(lm, "sonnet"):
		target = plan.Sonnet
	case strings.Contains(lm, "haiku"):
		target = plan.Haiku
	}
	if target == "" {
		target = fallback // R3 槽空回落 / R5 未知回落
	}
	if target == "" {
		return "", false
	}
	return target, true
}
