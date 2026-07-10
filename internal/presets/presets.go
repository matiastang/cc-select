// Package presets 提供内置的 Claude Code 服务商预设模板。
//
// 预设以 Go 代码形式编译进二进制，用户选择 preset 后只需填写少量必填字段
// （通常是 API Key），即可得到一份完整的 Claude settings.json env 配置。
package presets

import (
	"fmt"
	"regexp"
	"sort"
)

// APIFormat 表示 Claude Code 侧使用的请求协议格式。
type APIFormat string

const (
	APIFormatAnthropic       APIFormat = "anthropic"
	APIFormatOpenAIChat      APIFormat = "openai_chat"
	APIFormatOpenAIResponses APIFormat = "openai_responses"
	APIFormatGeminiNative    APIFormat = "gemini_native"
)

// AuthField 表示认证用的环境变量名。
type AuthField string

const (
	AuthFieldAuthToken AuthField = "ANTHROPIC_AUTH_TOKEN"
	AuthFieldAPIKey    AuthField = "ANTHROPIC_API_KEY"
)

// Preset 是单个内置服务商模板。
type Preset struct {
	ID           string            `json:"id"`
	DisplayName  string            `json:"displayName"`
	Category     Category          `json:"category"`
	WebsiteURL   string            `json:"websiteURL"`
	APIKeyURL    string            `json:"apiKeyURL"`
	APIFormat    APIFormat         `json:"apiFormat"`
	AuthField    AuthField         `json:"authField"`
	EnvTemplate  map[string]string `json:"envTemplate"`
	RequiredVars []string          `json:"requiredVars"`
	OptionalVars []string          `json:"optionalVars"`
	OAuth        bool              `json:"oauth"`
}

// Category 是供应商分类。
type Category string

const (
	CategoryOfficial   Category = "official"
	CategoryCNOfficial Category = "cn_official"
	CategoryAggregator Category = "aggregator"
	CategoryThirdParty Category = "third_party"
	CategoryCloud      Category = "cloud_provider"
	CategoryCustom     Category = "custom"
)

// placeholder 匹配 ${VAR} 语法，用于模板中标记用户必填项。
var placeholder = regexp.MustCompile(`\$\{([A-Za-z0-9_]+)\}`)

// builtin 是编译进二进制的预设表。顺序即展示顺序。
var builtin = []Preset{
	{
		ID:          "claude-official",
		DisplayName: "Claude 官方",
		Category:    CategoryOfficial,
		WebsiteURL:  "https://claude.ai",
		APIFormat:   APIFormatAnthropic,
		EnvTemplate: map[string]string{},
		OAuth:       true,
	},
	{
		ID:          "deepseek",
		DisplayName: "DeepSeek",
		Category:    CategoryCNOfficial,
		WebsiteURL:  "https://deepseek.com",
		APIKeyURL:   "https://platform.deepseek.com",
		APIFormat:   APIFormatAnthropic,
		AuthField:   AuthFieldAuthToken,
		EnvTemplate: map[string]string{
			"ANTHROPIC_BASE_URL":             "https://api.deepseek.com/anthropic",
			"ANTHROPIC_AUTH_TOKEN":           "${API_KEY}",
			"ANTHROPIC_MODEL":                "deepseek-v4-pro",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "deepseek-v4-flash",
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "deepseek-v4-pro",
			"ANTHROPIC_DEFAULT_OPUS_MODEL":   "deepseek-v4-pro",
		},
		RequiredVars: []string{"ANTHROPIC_AUTH_TOKEN"},
		OptionalVars: []string{"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL"},
	},
	{
		ID:          "zhipu-glm",
		DisplayName: "智谱 GLM",
		Category:    CategoryCNOfficial,
		WebsiteURL:  "https://open.bigmodel.cn",
		APIKeyURL:   "https://open.bigmodel.cn/usercenter/apikey",
		APIFormat:   APIFormatAnthropic,
		AuthField:   AuthFieldAuthToken,
		EnvTemplate: map[string]string{
			"ANTHROPIC_BASE_URL":             "https://open.bigmodel.cn/api/anthropic",
			"ANTHROPIC_AUTH_TOKEN":           "${API_KEY}",
			"ANTHROPIC_MODEL":                "glm-5.1",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "glm-5.1",
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "glm-5.1",
			"ANTHROPIC_DEFAULT_OPUS_MODEL":   "glm-5.1",
		},
		RequiredVars: []string{"ANTHROPIC_AUTH_TOKEN"},
		OptionalVars: []string{"ANTHROPIC_MODEL"},
	},
	{
		ID:          "zhipu-glm-en",
		DisplayName: "Zhipu GLM (en)",
		Category:    CategoryCNOfficial,
		WebsiteURL:  "https://api.z.ai",
		APIKeyURL:   "https://api.z.ai/settings",
		APIFormat:   APIFormatAnthropic,
		AuthField:   AuthFieldAuthToken,
		EnvTemplate: map[string]string{
			"ANTHROPIC_BASE_URL":             "https://api.z.ai/api/anthropic",
			"ANTHROPIC_AUTH_TOKEN":           "${API_KEY}",
			"ANTHROPIC_MODEL":                "glm-5.1",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "glm-5.1",
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "glm-5.1",
			"ANTHROPIC_DEFAULT_OPUS_MODEL":   "glm-5.1",
		},
		RequiredVars: []string{"ANTHROPIC_AUTH_TOKEN"},
		OptionalVars: []string{"ANTHROPIC_MODEL"},
	},
	{
		ID:          "kimi",
		DisplayName: "Kimi",
		Category:    CategoryCNOfficial,
		WebsiteURL:  "https://www.moonshot.cn",
		APIKeyURL:   "https://platform.moonshot.cn/console/api-keys",
		APIFormat:   APIFormatAnthropic,
		AuthField:   AuthFieldAuthToken,
		EnvTemplate: map[string]string{
			"ANTHROPIC_BASE_URL":             "https://api.moonshot.cn/anthropic",
			"ANTHROPIC_AUTH_TOKEN":           "${API_KEY}",
			"ANTHROPIC_MODEL":                "kimi-k2.7-code",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "kimi-k2.7-code",
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "kimi-k2.7-code",
			"ANTHROPIC_DEFAULT_OPUS_MODEL":   "kimi-k2.7-code",
		},
		RequiredVars: []string{"ANTHROPIC_AUTH_TOKEN"},
		OptionalVars: []string{"ANTHROPIC_MODEL"},
	},
	{
		ID:          "kimi-coding",
		DisplayName: "Kimi For Coding",
		Category:    CategoryCNOfficial,
		WebsiteURL:  "https://www.moonshot.cn",
		APIKeyURL:   "https://platform.moonshot.cn/console/api-keys",
		APIFormat:   APIFormatAnthropic,
		AuthField:   AuthFieldAuthToken,
		EnvTemplate: map[string]string{
			"ANTHROPIC_BASE_URL":             "https://api.kimi.com/coding/",
			"ANTHROPIC_AUTH_TOKEN":           "${API_KEY}",
			"ANTHROPIC_MODEL":                "kimi-k2.7-code",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "kimi-k2.7-code",
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "kimi-k2.7-code",
			"ANTHROPIC_DEFAULT_OPUS_MODEL":   "kimi-k2.7-code",
		},
		RequiredVars: []string{"ANTHROPIC_AUTH_TOKEN"},
		OptionalVars: []string{"ANTHROPIC_MODEL"},
	},
	{
		ID:          "openrouter",
		DisplayName: "OpenRouter",
		Category:    CategoryAggregator,
		WebsiteURL:  "https://openrouter.ai",
		APIKeyURL:   "https://openrouter.ai/keys",
		APIFormat:   APIFormatAnthropic,
		AuthField:   AuthFieldAuthToken,
		EnvTemplate: map[string]string{
			"ANTHROPIC_BASE_URL":             "https://openrouter.ai/api",
			"ANTHROPIC_AUTH_TOKEN":           "${API_KEY}",
			"ANTHROPIC_MODEL":                "anthropic/claude-sonnet-5",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "anthropic/claude-haiku-4.5",
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "anthropic/claude-sonnet-5",
			"ANTHROPIC_DEFAULT_OPUS_MODEL":   "anthropic/claude-opus-4.8",
		},
		RequiredVars: []string{"ANTHROPIC_AUTH_TOKEN"},
		OptionalVars: []string{"ANTHROPIC_MODEL"},
	},
	{
		ID:          "siliconflow",
		DisplayName: "SiliconFlow",
		Category:    CategoryAggregator,
		WebsiteURL:  "https://siliconflow.cn",
		APIKeyURL:   "https://cloud.siliconflow.cn/account/ak",
		APIFormat:   APIFormatAnthropic,
		AuthField:   AuthFieldAuthToken,
		EnvTemplate: map[string]string{
			"ANTHROPIC_BASE_URL":             "https://api.siliconflow.cn",
			"ANTHROPIC_AUTH_TOKEN":           "${API_KEY}",
			"ANTHROPIC_MODEL":                "Pro/MiniMaxAI/MiniMax-M2.7",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "Pro/MiniMaxAI/MiniMax-M2.7",
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "Pro/MiniMaxAI/MiniMax-M2.7",
			"ANTHROPIC_DEFAULT_OPUS_MODEL":   "Pro/MiniMaxAI/MiniMax-M2.7",
		},
		RequiredVars: []string{"ANTHROPIC_AUTH_TOKEN"},
		OptionalVars: []string{"ANTHROPIC_MODEL"},
	},
	{
		ID:          "volcano-agentplan",
		DisplayName: "火山 Agentplan",
		Category:    CategoryCNOfficial,
		WebsiteURL:  "https://www.volcengine.com/product/ark",
		APIKeyURL:   "https://console.volcengine.com/ark/apiKey",
		APIFormat:   APIFormatAnthropic,
		AuthField:   AuthFieldAuthToken,
		EnvTemplate: map[string]string{
			"ANTHROPIC_BASE_URL":             "https://ark.cn-beijing.volces.com/api/coding",
			"ANTHROPIC_AUTH_TOKEN":           "${API_KEY}",
			"ANTHROPIC_MODEL":                "ark-code-latest",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "ark-code-latest",
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "ark-code-latest",
			"ANTHROPIC_DEFAULT_OPUS_MODEL":   "ark-code-latest",
		},
		RequiredVars: []string{"ANTHROPIC_AUTH_TOKEN"},
		OptionalVars: []string{"ANTHROPIC_MODEL"},
	},
	{
		ID:          "aws-bedrock-aksk",
		DisplayName: "AWS Bedrock (AKSK)",
		Category:    CategoryCloud,
		WebsiteURL:  "https://aws.amazon.com/bedrock",
		APIFormat:   APIFormatAnthropic,
		AuthField:   AuthFieldAuthToken,
		EnvTemplate: map[string]string{
			"ANTHROPIC_BASE_URL":             "https://bedrock-runtime.${AWS_REGION}.amazonaws.com",
			"ANTHROPIC_AUTH_TOKEN":           "${API_KEY}",
			"AWS_ACCESS_KEY_ID":              "${AWS_ACCESS_KEY_ID}",
			"AWS_SECRET_ACCESS_KEY":          "${AWS_SECRET_ACCESS_KEY}",
			"AWS_REGION":                     "${AWS_REGION}",
			"ANTHROPIC_MODEL":                "global.anthropic.claude-opus-4-8",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "global.anthropic.claude-haiku-4.5-20251001-v1:0",
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "global.anthropic.claude-sonnet-5",
			"ANTHROPIC_DEFAULT_OPUS_MODEL":   "global.anthropic.claude-opus-4-8",
			"CLAUDE_CODE_USE_BEDROCK":        "1",
		},
		RequiredVars: []string{"ANTHROPIC_AUTH_TOKEN", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION"},
		OptionalVars: []string{"ANTHROPIC_MODEL", "AWS_REGION"},
	},
	{
		ID:          "aws-bedrock-apikey",
		DisplayName: "AWS Bedrock (API Key)",
		Category:    CategoryCloud,
		WebsiteURL:  "https://aws.amazon.com/bedrock",
		APIFormat:   APIFormatAnthropic,
		AuthField:   AuthFieldAPIKey,
		EnvTemplate: map[string]string{
			"ANTHROPIC_BASE_URL":             "https://bedrock-runtime.${AWS_REGION}.amazonaws.com",
			"ANTHROPIC_API_KEY":              "${API_KEY}",
			"AWS_REGION":                     "${AWS_REGION}",
			"ANTHROPIC_MODEL":                "global.anthropic.claude-opus-4-8",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "global.anthropic.claude-haiku-4.5-20251001-v1:0",
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "global.anthropic.claude-sonnet-5",
			"ANTHROPIC_DEFAULT_OPUS_MODEL":   "global.anthropic.claude-opus-4-8",
			"CLAUDE_CODE_USE_BEDROCK":        "1",
		},
		RequiredVars: []string{"ANTHROPIC_API_KEY", "AWS_REGION"},
		OptionalVars: []string{"ANTHROPIC_MODEL", "AWS_REGION"},
	},
	{
		ID:          "github-copilot",
		DisplayName: "GitHub Copilot",
		Category:    CategoryThirdParty,
		WebsiteURL:  "https://github.com/copilot",
		APIFormat:   APIFormatOpenAIChat,
		EnvTemplate: map[string]string{
			"ANTHROPIC_BASE_URL":             "https://api.githubcopilot.com",
			"ANTHROPIC_MODEL":                "claude-sonnet-5",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "claude-haiku-4.5",
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "claude-sonnet-5",
			"ANTHROPIC_DEFAULT_OPUS_MODEL":   "claude-sonnet-5",
		},
		OAuth:        true,
		RequiredVars: []string{},
	},
	{
		ID:          "custom",
		DisplayName: "自定义配置",
		Category:    CategoryCustom,
		APIFormat:   APIFormatAnthropic,
		AuthField:   AuthFieldAuthToken,
		EnvTemplate: map[string]string{
			"ANTHROPIC_BASE_URL":   "",
			"ANTHROPIC_AUTH_TOKEN": "${API_KEY}",
			"ANTHROPIC_MODEL":      "",
		},
		RequiredVars: []string{"ANTHROPIC_AUTH_TOKEN"},
		OptionalVars: []string{"ANTHROPIC_BASE_URL", "ANTHROPIC_MODEL"},
	},
}

// byID 是 built-in 表的索引，init 时构建以保证查找性能。
var byID map[string]Preset

func init() {
	byID = make(map[string]Preset, len(builtin))
	for _, p := range builtin {
		byID[p.ID] = p
	}
}

// All 返回所有内置 preset 的副本。顺序与内置表一致。
func All() []Preset {
	out := make([]Preset, len(builtin))
	copy(out, builtin)
	return out
}

// ByID 按 id 查找 preset。
func ByID(id string) (Preset, bool) {
	p, ok := byID[id]
	return p, ok
}

// Categories 返回去重且按固定顺序排列的分类列表，
// 仅包含当前实际存在的分类。
func Categories() []Category {
	order := []Category{
		CategoryOfficial,
		CategoryCNOfficial,
		CategoryCloud,
		CategoryAggregator,
		CategoryThirdParty,
		CategoryCustom,
	}
	seen := map[Category]struct{}{}
	for _, p := range builtin {
		seen[p.Category] = struct{}{}
	}
	out := make([]Category, 0, len(seen))
	for _, c := range order {
		if _, ok := seen[c]; ok {
			out = append(out, c)
		}
	}
	return out
}

// AllByCategory 按分类分组返回 preset。
func AllByCategory() map[Category][]Preset {
	out := make(map[Category][]Preset)
	for _, p := range builtin {
		out[p.Category] = append(out[p.Category], p)
	}
	return out
}

// Apply 将 preset 模板与用户覆盖值合并成最终 env map。
//   - overrides 中值为空串的字段会被忽略（保留模板默认值/占位符）。
//   - 返回的是新 map，不修改模板。
func Apply(p Preset, overrides map[string]string) map[string]string {
	env := make(map[string]string, len(p.EnvTemplate))
	for k, v := range p.EnvTemplate {
		env[k] = v
	}
	for k, v := range overrides {
		if v == "" {
			continue
		}
		env[k] = v
	}
	return env
}

// Expand 将 env 中的 ${VAR} 占位符按 provided 替换。
// 返回替换后的 env 与仍未替换的占位符变量名列表（去重排序）。
func Expand(env map[string]string, provided map[string]string) (map[string]string, []string) {
	out := make(map[string]string, len(env))
	missingSet := map[string]struct{}{}
	for k, v := range env {
		out[k] = placeholder.ReplaceAllStringFunc(v, func(match string) string {
			name := match[2 : len(match)-1] // ${NAME} -> NAME
			if val, ok := provided[name]; ok && val != "" {
				return val
			}
			missingSet[name] = struct{}{}
			return match
		})
	}
	missing := make([]string, 0, len(missingSet))
	for name := range missingSet {
		missing = append(missing, name)
	}
	sort.Strings(missing)
	return out, missing
}

// RequiredMissing 返回 requiredVars 中缺失或仍为 ${VAR} 占位符的 key。
func RequiredMissing(p Preset, env map[string]string) []string {
	var missing []string
	for _, key := range p.RequiredVars {
		v, ok := env[key]
		if !ok || v == "" {
			missing = append(missing, key)
			continue
		}
		if placeholder.MatchString(v) {
			missing = append(missing, key)
		}
	}
	return missing
}

// PlaceholdersIn 返回 env 中所有未替换的占位符变量名（去重排序）。
func PlaceholdersIn(env map[string]string) []string {
	_, missing := Expand(env, nil)
	return missing
}

// FormatMissing 将缺失字段格式化为可读字符串。
func FormatMissing(missing []string) string {
	switch len(missing) {
	case 0:
		return ""
	case 1:
		return missing[0]
	default:
		out := ""
		for i, m := range missing {
			if i > 0 {
				out += ", "
			}
			out += m
		}
		return out
	}
}

// BuildEnv 是 CLI/Web 共用的便捷函数：取 preset → 合并覆盖 → 展开占位符。
// apiKey 仅在 preset 非 OAuth 且 authField 含 ${API_KEY} 占位符时参与填充。
// 返回最终 env 与未满足的必填字段列表（为空表示成功）。
func BuildEnv(presetID, apiKey string, overrides map[string]string) (map[string]string, []string, error) {
	p, ok := ByID(presetID)
	if !ok {
		return nil, nil, fmt.Errorf("preset not found: %s", presetID)
	}
	env := Apply(p, overrides)
	provided := make(map[string]string, len(overrides)+1)
	for k, v := range overrides {
		provided[k] = v
	}
	if !p.OAuth && apiKey != "" {
		// 把 API Key 注入到认证字段。
		provided["API_KEY"] = apiKey
		env[string(p.AuthField)] = apiKey
	}
	expanded, _ := Expand(env, provided)
	missing := RequiredMissing(p, expanded)
	return expanded, missing, nil
}
