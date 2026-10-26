// Package usage 提供限额查询（specs/004 US7 / R11 / FR-018-020）：按 provider 的
// base URL 匹配适配器（GLM 专用端点 / Kimi 余额 / 通用 Anthropic 限流头探测），
// 返回归一化的窗口列表。仅手动触发——本包不做任何缓存与轮询。
package usage

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/cc-select/cc-select/internal/probe"
)

// 窗口类型（data-model §2.5）。
const (
	Window5h             = "5h"
	WindowWeek           = "week"
	WindowDay            = "day"
	WindowMinuteRequests = "minute-requests"
	WindowMinuteTokens   = "minute-tokens"
	WindowMonthMCP       = "month-mcp"
	WindowBalance        = "balance"
)

// Window 是归一化限额窗口：GLM 仅有百分比（Percent），Anthropic 系有 used/limit/reset。
type Window struct {
	Type    string     `json:"type"`
	Used    *float64   `json:"used,omitempty"`
	Limit   *float64   `json:"limit,omitempty"`
	Percent *float64   `json:"percent,omitempty"`
	ResetAt *time.Time `json:"resetAt,omitempty"`
}

// Report 是一次查询的结果（API 载荷真值，data-model §2.5）。
type Report struct {
	Supported  bool      `json:"supported"`
	ProviderID string    `json:"providerId,omitempty"`
	Windows    []Window  `json:"windows,omitempty"`
	FetchedAt  time.Time `json:"fetchedAt"`
	Reason     string    `json:"reason,omitempty"` // unsupported 原因（非错误）
}

// Adapter 是 per-provider 查询策略（contracts/presets-and-usage §2.1）。
type Adapter interface {
	ID() string
	Match(baseURL string) bool
	Query(ctx context.Context, env map[string]string, opts *probe.Options) (*Report, error)
}

// registry 按声明序匹配，首个命中生效；兜底适配器恒匹配。
var registry = []Adapter{
	glmQuota{},
	kimiBalance{},
	anthropicHeaders{},
}

func matchAdapter(baseURL string) Adapter {
	for _, a := range registry {
		if a.Match(baseURL) {
			return a
		}
	}
	return nil
}

// Query 对给定 env 执行限额查询。错误（网络/服务端 5xx）以 error 返回并已脱敏；
// 「服务商不支持」是正常结果（Supported=false + Reason），不是 error。
func Query(ctx context.Context, env map[string]string) (*Report, error) {
	a := matchAdapter(env["ANTHROPIC_BASE_URL"])
	if a == nil {
		return nil, nil // 理论不可达：兜底适配器恒匹配
	}
	return a.Query(ctx, env, nil)
}

// hostOf 提取 base URL 的 host（小写）；解析失败返回空串。
func hostOf(baseURL string) string {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Host)
}

func f64(v float64) *float64 { return &v }
