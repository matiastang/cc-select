package presets

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// SchemaVersion 是当前编译期支持的 presets bundle schema 版本。
// 远端/缓存 bundle 版本不匹配时整包拒绝（data-model §1.1）。
const SchemaVersion = 1

// Bundle 是 presets.json 的顶层结构。
type Bundle struct {
	SchemaVersion int      `json:"schemaVersion"`
	Presets       []Preset `json:"presets"`
}

var validCategories = map[Category]bool{
	CategoryOfficial:   true,
	CategoryCNOfficial: true,
	CategoryAggregator: true,
	CategoryThirdParty: true,
	CategoryCloud:      true,
	CategoryCustom:     true,
}

var validAPIFormats = map[APIFormat]bool{
	APIFormatAnthropic:       true,
	APIFormatOpenAIChat:      true,
	APIFormatOpenAIResponses: true,
	APIFormatGeminiNative:    true,
}

var validAuthFields = map[AuthField]bool{
	AuthFieldAuthToken: true,
	AuthFieldAPIKey:    true,
}

// Validate 校验一份 bundle JSON（整包语义：任一条不合法即拒绝）。
// 返回解析后的 Bundle 便于调用方直接启用。
func Validate(data []byte) error {
	var b Bundle
	if err := json.Unmarshal(data, &b); err != nil {
		return fmt.Errorf("presets: parse bundle: %w", err)
	}
	if b.SchemaVersion != SchemaVersion {
		return fmt.Errorf("presets: schemaVersion %d unsupported (want %d)", b.SchemaVersion, SchemaVersion)
	}
	if len(b.Presets) == 0 {
		return fmt.Errorf("presets: empty bundle")
	}

	seen := map[string]struct{}{}
	for i, p := range b.Presets {
		if p.ID == "" {
			return fmt.Errorf("presets[%d]: id is empty", i)
		}
		if _, dup := seen[p.ID]; dup {
			return fmt.Errorf("presets[%d]: duplicate id %q", i, p.ID)
		}
		seen[p.ID] = struct{}{}

		if p.DisplayName == "" {
			return fmt.Errorf("presets[%d] %q: displayName is empty", i, p.ID)
		}
		if !validCategories[p.Category] {
			return fmt.Errorf("presets[%d] %q: unknown category %q", i, p.ID, p.Category)
		}
		if !validAPIFormats[p.APIFormat] {
			return fmt.Errorf("presets[%d] %q: unknown apiFormat %q", i, p.ID, p.APIFormat)
		}
		if !p.OAuth {
			if !validAuthFields[p.AuthField] {
				return fmt.Errorf("presets[%d] %q: authField %q invalid (non-oauth preset must declare one)", i, p.ID, p.AuthField)
			}
		}
		if err := checkHTTPS("websiteURL", p.WebsiteURL); err != nil {
			return fmt.Errorf("presets[%d] %q: %w", i, p.ID, err)
		}
		if err := checkHTTPS("apiKeyURL", p.APIKeyURL); err != nil {
			return fmt.Errorf("presets[%d] %q: %w", i, p.ID, err)
		}
		if err := checkPlaceholders(p, i); err != nil {
			return err
		}
		// 宪法 VI：预设决定 token 去向——env 模板里的 base URL 若非空必须 https。
		if base := p.EnvTemplate["ANTHROPIC_BASE_URL"]; base != "" && !strings.HasPrefix(base, "https://") {
			return fmt.Errorf("presets[%d] %q: envTemplate ANTHROPIC_BASE_URL must be https", i, p.ID)
		}
	}
	return nil
}

// checkPlaceholders：envTemplate 中每个 ${VAR} 必须被 requiredVars ∪ optionalVars ∪ {API_KEY}
// 之一声明，避免出现用户永远无法通过表单填满的模板。
func checkPlaceholders(p Preset, i int) error {
	declared := map[string]struct{}{"API_KEY": {}}
	for _, v := range p.RequiredVars {
		declared[v] = struct{}{}
	}
	for _, v := range p.OptionalVars {
		declared[v] = struct{}{}
	}
	names := map[string]struct{}{}
	for _, v := range p.EnvTemplate {
		for _, m := range placeholder.FindAllStringSubmatch(v, -1) {
			names[m[1]] = struct{}{}
		}
	}
	var unknown []string
	for n := range names {
		if _, ok := declared[n]; !ok {
			unknown = append(unknown, n)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("presets[%d] %q: undeclared placeholder(s): %s", i, p.ID, strings.Join(unknown, ", "))
	}
	return nil
}

func checkHTTPS(field, raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s %q unparseable: %w", field, raw, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%s %q must be https", field, raw)
	}
	return nil
}
