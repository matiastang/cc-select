package usage

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cc-select/cc-select/internal/probe"
)

// anthropicHeaders 是兜底适配器：对任意 provider 发 max_tokens=1 的最小请求并解析
// Anthropic 系限流响应头。兼容两种形态（research §2.3 防御式解析）：
//
//  1. 独立头：anthropic-ratelimit-unified-{5h,week}-{limit,remaining,reset}
//     与分钟级 anthropic-ratelimit-{requests,tokens}-{limit,remaining,reset}
//  2. 组合头：anthropic-ratelimit-unified-quota 等含 k=v 列表的载荷（扫描 5h/week 键）
type anthropicHeaders struct{}

func (anthropicHeaders) ID() string { return "anthropic-headers" }

func (anthropicHeaders) Match(string) bool { return true } // 兜底恒匹配

func (anthropicHeaders) Query(ctx context.Context, env map[string]string, opts *probe.Options) (*Report, error) {
	baseURL := strings.TrimSpace(env["ANTHROPIC_BASE_URL"])
	if baseURL == "" {
		// 官方形态（contracts/api.md §2.4）：无自定义 base → 探测官方端点。
		baseURL = probe.OfficialBaseURL
	}
	token := strings.TrimSpace(env["ANTHROPIC_AUTH_TOKEN"])
	apiKey := strings.TrimSpace(env["ANTHROPIC_API_KEY"])
	model := strings.TrimSpace(env["ANTHROPIC_MODEL"])
	if model == "" {
		model = probe.OfficialDefaultModel // 探测用模型：只读响应头
	}

	status, header, _, err := probe.SendMessage(ctx, opts, baseURL, token, apiKey, model)
	if err != nil {
		return nil, sanitizeErr(err, token, apiKey)
	}
	if status >= 500 {
		return nil, fmt.Errorf("provider status %d", status)
	}
	if status == 401 || status == 403 {
		return nil, fmt.Errorf("auth failed (%d)", status)
	}

	now := time.Now().UTC()
	windows := parseUnifiedHeaders(header)
	windows = append(windows, parseMinuteHeaders(header)...)

	if len(windows) == 0 {
		return &Report{Supported: false, FetchedAt: now, Reason: "no_limit_headers"}, nil
	}
	return &Report{Supported: true, Windows: windows, FetchedAt: now}, nil
}

// parseUnifiedHeaders 解析独立 unified 头（形态一）+ 扫描组合头（形态二）。
func parseUnifiedHeaders(h http.Header) []Window {
	var out []Window
	for _, window := range []struct {
		label string
		typ   string
	}{
		{"5h", Window5h},
		{"week", WindowWeek},
		{"7d", WindowWeek},
	} {
		limit, hasLimit := headerFloat(h, "anthropic-ratelimit-unified-"+window.label+"-limit")
		remaining, hasRemaining := headerFloat(h, "anthropic-ratelimit-unified-"+window.label+"-remaining")
		if !hasLimit && !hasRemaining {
			continue
		}
		w := Window{Type: window.typ}
		if hasLimit {
			w.Limit = f64(limit)
			if hasRemaining {
				w.Used = f64(limit - remaining)
			}
		}
		if reset := headerTime(h, "anthropic-ratelimit-unified-"+window.label+"-reset"); reset != nil {
			w.ResetAt = reset
		}
		out = append(out, w)
	}
	// 形态二：组合头里的 k=v（如 "5h=63%;week=20%"）——仅百分比可用。
	for _, key := range []string{
		"anthropic-ratelimit-unified-quota",
		"anthropic-ratelimit-unified-status",
		"anthropic-ratelimit-unified",
	} {
		for _, raw := range h.Values(key) {
			out = append(out, parseKVPercentages(raw)...)
		}
	}
	return dedupeWindows(out)
}

// parseKVPercentages 扫描 "k=v; k2=v2" 载荷中含 5h/week 键的百分比。
func parseKVPercentages(raw string) []Window {
	var out []Window
	for _, pair := range strings.FieldsFunc(raw, func(r rune) bool { return r == ';' || r == ',' }) {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		typ := ""
		switch {
		case strings.Contains(key, "5h"):
			typ = Window5h
		case strings.Contains(key, "week") || strings.Contains(key, "7d"):
			typ = WindowWeek
		default:
			continue
		}
		val := strings.TrimSuffix(strings.TrimSpace(kv[1]), "%")
		if p, err := strconv.ParseFloat(val, 64); err == nil {
			out = append(out, Window{Type: typ, Percent: f64(p)})
		}
	}
	return out
}

func parseMinuteHeaders(h http.Header) []Window {
	var out []Window
	for _, m := range []struct {
		prefix string
		typ    string
	}{
		{"anthropic-ratelimit-requests", WindowMinuteRequests},
		{"anthropic-ratelimit-tokens", WindowMinuteTokens},
	} {
		limit, hasLimit := headerFloat(h, m.prefix+"-limit")
		remaining, hasRemaining := headerFloat(h, m.prefix+"-remaining")
		if !hasLimit && !hasRemaining {
			continue
		}
		w := Window{Type: m.typ}
		if hasLimit {
			w.Limit = f64(limit)
			if hasRemaining {
				w.Used = f64(limit - remaining)
			}
		}
		if reset := headerTime(h, m.prefix+"-reset"); reset != nil {
			w.ResetAt = reset
		}
		out = append(out, w)
	}
	return out
}

func headerFloat(h http.Header, key string) (float64, bool) {
	raw := strings.TrimSpace(h.Get(key))
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

var timeLayouts = []string{time.RFC1123, time.RFC1123Z, time.RFC3339}

func headerTime(h http.Header, key string) *time.Time {
	raw := strings.TrimSpace(h.Get(key))
	if raw == "" {
		return nil
	}
	// 支持秒级 Unix 时间戳与标准日期格式两种。
	if sec, err := strconv.ParseInt(raw, 10, 64); err == nil {
		t := time.Unix(sec, 0).UTC()
		return &t
	}
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			tt := t.UTC()
			return &tt
		}
	}
	return nil
}

// dedupeWindows 按类型去重（独立头优先于组合头扫描）。
func dedupeWindows(in []Window) []Window {
	seen := map[string]struct{}{}
	out := make([]Window, 0, len(in))
	for _, w := range in {
		if _, ok := seen[w.Type]; ok {
			continue
		}
		seen[w.Type] = struct{}{}
		out = append(out, w)
	}
	return out
}

func sanitizeErr(err error, secrets ...string) error {
	msg := err.Error()
	for _, s := range secrets {
		if s != "" {
			msg = strings.ReplaceAll(msg, s, "***")
		}
	}
	return fmt.Errorf("%s", msg)
}
