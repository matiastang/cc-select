// Package probe 提供「测试连接」能力（specs/004 US6 / FR-016-017）：以给定 env
// 向服务商发一个 max_tokens=1 的最小 /v1/messages 请求，返回结构化结果。
// 仅由用户手动触发；R11 限额查询的头探测复用本包的共享客户端。
package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// OfficialBaseURL 是无自定义 base URL（官方形态）的探测目标。
const OfficialBaseURL = "https://api.anthropic.com"

// OfficialDefaultModel 官方形态且未配置模型时的安全兜底（官方目录必有）。
const OfficialDefaultModel = "claude-sonnet-4-5"

const probeTimeout = 10 * time.Second
const maxRespBody = 64 << 10

// Options 注入依赖（测试替身）。
type Options struct {
	Timeout time.Duration
	Client  *http.Client
}

// ErrorKind 是探测失败的分类（contracts/api.md §2.3）。
type ErrorKind string

const (
	ErrInvalid ErrorKind = "invalid" // 请求前提不满足（缺 key/缺模型）——API 层转 400
	ErrAuth    ErrorKind = "auth"
	ErrModel   ErrorKind = "model"
	ErrServer  ErrorKind = "server"
	ErrNetwork ErrorKind = "network"
	ErrUnknown ErrorKind = "unknown"
)

// ProbeError 携带分类与脱敏消息。
type ProbeError struct {
	Kind    ErrorKind `json:"kind"`
	Message string    `json:"message"`
}

// Result 是探测结果（API 载荷真值结构，data-model §2.4）。
type Result struct {
	OK        bool        `json:"ok"`
	LatencyMs int64       `json:"latencyMs"`
	Model     string      `json:"model,omitempty"`
	Error     *ProbeError `json:"error,omitempty"`
}

// Probe 执行一次连通性测试。env 至少含认证字段之一；官方形态（无 base URL）
// 模型可缺省，自定义 base URL 必须显式模型。
func Probe(ctx context.Context, env map[string]string, opts *Options) Result {
	o := optionsOrDefault(opts)

	baseURL := strings.TrimSpace(env["ANTHROPIC_BASE_URL"])
	model := strings.TrimSpace(env["ANTHROPIC_MODEL"])

	authToken := strings.TrimSpace(env["ANTHROPIC_AUTH_TOKEN"])
	apiKey := strings.TrimSpace(env["ANTHROPIC_API_KEY"])
	if authToken == "" && apiKey == "" {
		return fail(ErrInvalid, "missing auth field (ANTHROPIC_AUTH_TOKEN or ANTHROPIC_API_KEY)")
	}
	if model == "" {
		if baseURL == "" {
			model = OfficialDefaultModel
		} else {
			return fail(ErrInvalid, "missing ANTHROPIC_MODEL for custom base URL")
		}
	}
	if baseURL == "" {
		baseURL = OfficialBaseURL
	}

	start := time.Now()
	status, _, body, err := SendMessage(ctx, o, baseURL, authToken, apiKey, model)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return Result{OK: false, LatencyMs: latency, Error: classify(err, 0, "", secrets(authToken, apiKey))}
	}

	if status == http.StatusOK {
		var resp struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(body, &resp)
		echo := resp.Model
		if echo == "" {
			echo = model
		}
		return Result{OK: true, LatencyMs: latency, Model: echo}
	}
	pe := classify(nil, status, string(body), secrets(authToken, apiKey))
	return Result{OK: false, LatencyMs: latency, Error: pe}
}

// SendMessage 是共享最小 Anthropic 客户端（R11 头探测复用）：POST {base}/v1/messages。
// 认证双头并带：AUTH_TOKEN → Authorization Bearer；API_KEY → x-api-key（服务商按需取用）。
func SendMessage(ctx context.Context, o *Options, baseURL, authToken, apiKey, model string) (int, http.Header, []byte, error) {
	payload, _ := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	}
	var client *http.Client
	if o != nil && o.Client != nil {
		client = o.Client
	} else {
		timeout := probeTimeout
		if o != nil && o.Timeout > 0 {
			timeout = o.Timeout
		}
		client = &http.Client{Timeout: timeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBody))
	if err != nil {
		return resp.StatusCode, resp.Header, nil, err
	}
	return resp.StatusCode, resp.Header, body, nil
}

func classify(err error, status int, body string, secrets []string) *ProbeError {
	msg := func(s string) string {
		s = strings.TrimSpace(s)
		if len(s) > 200 {
			s = s[:200]
		}
		for _, sec := range secrets {
			if sec != "" {
				s = strings.ReplaceAll(s, sec, "***")
			}
		}
		return s
	}

	if err != nil {
		var nerr net.Error
		if errors.As(err, &nerr) || errors.Is(err, context.DeadlineExceeded) {
			return &ProbeError{Kind: ErrNetwork, Message: msg("timeout: " + err.Error())}
		}
		return &ProbeError{Kind: ErrNetwork, Message: msg(err.Error())}
	}

	lower := strings.ToLower(body)
	switch {
	case status == 401 || status == 403:
		return &ProbeError{Kind: ErrAuth, Message: msg(body)}
	case status == 404, status == 400 && (strings.Contains(lower, "model") || strings.Contains(lower, "not found")):
		return &ProbeError{Kind: ErrModel, Message: msg(body)}
	case status >= 500:
		return &ProbeError{Kind: ErrServer, Message: msg(body)}
	default:
		return &ProbeError{Kind: ErrUnknown, Message: msg(body)}
	}
}

func optionsOrDefault(o *Options) *Options {
	if o == nil {
		return &Options{Timeout: probeTimeout}
	}
	if o.Timeout <= 0 {
		o.Timeout = probeTimeout
	}
	return o
}

func fail(kind ErrorKind, msg string) Result {
	return Result{OK: false, Error: &ProbeError{Kind: kind, Message: msg}}
}

func secrets(authToken, apiKey string) []string {
	return []string{authToken, apiKey}
}

// defaultModelFor 暴露给测试验证官方兜底逻辑。
func defaultModelFor(baseURL string) string {
	if baseURL == "" {
		return OfficialDefaultModel
	}
	return ""
}
