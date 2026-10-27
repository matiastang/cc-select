package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cc-select/cc-select/internal/probe"
)

const (
	maxUsageBody = 128 << 10
	usageTimeout = 10 * time.Second
	balancePath  = "/v1/users/me/balance"
)

// kimiBalance 适配 Kimi 开放平台余额端点（research §2.3）：GET {base}/v1/users/me/balance，
// Authorization: Bearer <token>。仅开放平台 host（api.moonshot.cn/.com）注册；
// api.kimi.com（For Coding）无公开端点——不注册，落兜底头探测后如实显示不支持。
type kimiBalance struct{}

func (kimiBalance) ID() string { return "kimi-balance" }

func (kimiBalance) Match(baseURL string) bool {
	host := hostOf(baseURL)
	return host == "api.moonshot.cn" || host == "api.moonshot.com"
}

func (kimiBalance) Query(ctx context.Context, env map[string]string, opts *probe.Options) (*Report, error) {
	token := strings.TrimSpace(env["ANTHROPIC_AUTH_TOKEN"])
	if token == "" {
		return nil, fmt.Errorf("missing ANTHROPIC_AUTH_TOKEN")
	}
	base := strings.TrimRight(strings.TrimSpace(env["ANTHROPIC_BASE_URL"]), "/")
	endpoint := base + balancePath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClientOf(opts, probeTimeoutOf(opts)).Do(req)
	if err != nil {
		return nil, sanitizeErr(err, token)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUsageBody))
	if err != nil {
		return nil, sanitizeErr(err, token)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, sanitizeErr(fmt.Errorf("balance endpoint status %d", resp.StatusCode), token)
	}

	var payload struct {
		AvailableBalance float64 `json:"available_balance"`
		Balance          float64 `json:"balance"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, sanitizeErr(err, token)
	}

	now := time.Now().UTC()
	used := payload.Balance - payload.AvailableBalance
	if used < 0 {
		used = 0
	}
	return &Report{
		Supported: true,
		FetchedAt: now,
		Windows: []Window{
			{Type: WindowBalance, Used: f64(used), Limit: f64(payload.Balance)},
		},
	}, nil
}

func probeTimeoutOf(opts *probe.Options) time.Duration {
	if opts != nil && opts.Timeout > 0 {
		return opts.Timeout
	}
	return usageTimeout
}

func httpClientOf(opts *probe.Options, timeout time.Duration) *http.Client {
	if opts != nil && opts.Client != nil {
		return opts.Client
	}
	return &http.Client{Timeout: timeout}
}
