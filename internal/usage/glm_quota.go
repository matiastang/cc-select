package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cc-select/cc-select/internal/probe"
)

// glmQuota 适配 GLM Coding Plan 官方配额端点（research §2.3，源自官方插件
// zai-org/zai-coding-plugins query-usage.mjs 的实证）：
//
//	GET {scheme}://{host}/api/monitor/usage/quota/limit
//	Authorization: <ANTHROPIC_AUTH_TOKEN>（原样，无 Bearer 前缀）
//	响应 data.limits[]：{type, percentage, currentValue, usage, usageDetails}
//	TOKENS_LIMIT → 5 小时窗口（仅百分比）；TIME_LIMIT → 月度 MCP 用量（仅百分比）。
type glmQuota struct{}

// GLMAdapter 是 GLM 配额适配器的导出实例（web 层测试注入用）。
var GLMAdapter Adapter = glmQuota{}

func (glmQuota) ID() string { return "glm" }

func (glmQuota) Match(baseURL string) bool {
	host := hostOf(baseURL)
	return host == "open.bigmodel.cn" || host == "api.z.ai" || host == "dev.bigmodel.cn"
}

func (glmQuota) Query(ctx context.Context, env map[string]string, opts *probe.Options) (*Report, error) {
	token := strings.TrimSpace(env["ANTHROPIC_AUTH_TOKEN"])
	if token == "" {
		return nil, fmt.Errorf("missing ANTHROPIC_AUTH_TOKEN")
	}
	base := strings.TrimRight(strings.TrimSpace(env["ANTHROPIC_BASE_URL"]), "/")
	endpoint, err := rebuildEndpoint(base, "/api/monitor/usage/quota/limit")
	if err != nil {
		return nil, err
	}

	timeout := probeTimeoutOf(opts)
	client := httpClientOf(opts, timeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Accept-Language", "en-US,en")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, sanitizeErr(err, token)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUsageBody))
	if err != nil {
		return nil, sanitizeErr(err, token)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, sanitizeErr(fmt.Errorf("quota endpoint status %d", resp.StatusCode), token)
	}

	var payload struct {
		Data struct {
			Limits []struct {
				Type       string  `json:"type"`
				Percentage float64 `json:"percentage"`
			} `json:"limits"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, sanitizeErr(err, token)
	}

	now := time.Now().UTC()
	report := &Report{Supported: true, FetchedAt: now}
	for _, l := range payload.Data.Limits {
		switch strings.ToUpper(strings.TrimSpace(l.Type)) {
		case "TOKENS_LIMIT":
			report.Windows = append(report.Windows, Window{Type: Window5h, Percent: f64(l.Percentage)})
		case "TIME_LIMIT":
			report.Windows = append(report.Windows, Window{Type: WindowMonthMCP, Percent: f64(l.Percentage)})
		}
	}
	if len(report.Windows) == 0 {
		// 端点可达但无窗口数据：视为不支持（套餐类型差异），不误报用量。
		report.Supported = false
		report.Reason = "no_limit_data"
	}
	return report, nil
}

// rebuildEndpoint 以 base URL 的 scheme+host 拼接监控端点（路径与 base 的 /api/anthropic 无关）。
func rebuildEndpoint(base, path string) (string, error) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid base URL: %s", base)
	}
	return u.Scheme + "://" + u.Host + path, nil
}
