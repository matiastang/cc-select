// Package presets 提供内置的 Claude Code 服务商预设模板。
//
// 预设以 Go 代码形式编译进二进制，用户选择 preset 后只需填写少量必填字段
// （通常是 API Key），即可得到一份完整的 Claude settings.json env 配置。
package presets

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"sync"
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

//go:embed presets.json
var embeddedPresets []byte

// store 是当前生效的预设表。进程启动时加载 embed 的 presets.json；
// reloadStore 供远端预设（specs/004 US5）整包热替换——校验通过才可调用。
var (
	storeMu sync.RWMutex
	store   []Preset
	storeID map[string]Preset
)

func reloadStore(b Bundle) {
	index := make(map[string]Preset, len(b.Presets))
	for _, p := range b.Presets {
		index[p.ID] = p
	}
	storeMu.Lock()
	store = b.Presets
	storeID = index
	storeMu.Unlock()
}

func init() {
	// embed 文件随仓库走 CI 校验（presets-lint）；不合法即编译期资产损坏，panic 最响亮。
	if err := Validate(embeddedPresets); err != nil {
		panic("presets: embedded presets.json invalid: " + err.Error())
	}
	var b Bundle
	if err := json.Unmarshal(embeddedPresets, &b); err != nil {
		panic("presets: embedded presets.json unparseable: " + err.Error())
	}
	reloadStore(b)
}

// All 返回当前生效 preset 的副本（顺序即 bundle 内顺序）。
func All() []Preset {
	storeMu.RLock()
	defer storeMu.RUnlock()
	out := make([]Preset, len(store))
	copy(out, store)
	return out
}

// ByID 按 id 查找 preset。
func ByID(id string) (Preset, bool) {
	storeMu.RLock()
	defer storeMu.RUnlock()
	p, ok := storeID[id]
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
	for _, p := range All() {
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
	for _, p := range All() {
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
