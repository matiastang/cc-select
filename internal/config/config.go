// Package config 负责读写 cc-select 的 provider 配置。
//
// 配置存于单个 JSON 文件（~/.cc-select/providers.json），采用原子写
// （写临时文件 + rename）保证多进程并发安全。详见 docs/architecture.md §3。
//
// API key 不以明文落盘：JSON 里存形如 "$keychain:<service>" 的占位，
// 真值由 internal/secrets 从系统 Keychain 取。见 docs/tech-stack.md §4。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// KeychainPlaceholderPrefix 标记一个 env 值是 Keychain 引用而非明文。
// 例："$keychain:cc-select:glm:ANTHROPIC_AUTH_TOKEN" → 从 Keychain 取该 service 的值。
const KeychainPlaceholderPrefix = "$keychain:"

// validID 限定 provider ID 的合法字符。ID 会被拼进文件系统路径
// （~/.cc-select/profiles/<id>），故必须排除 /、\、空白等可造成路径穿越的字符。
var validID = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ValidateID 校验 provider ID 是否安全可用作目录名。
// 仅允许 [A-Za-z0-9._-]，且拒绝 "." 与 ".."（它们能通过字符白名单却指向上级目录）。
// 这是防路径穿越的唯一关口——所有写入/删除 profile 的入口都应先调用它。
func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("provider id 不能为空")
	}
	if id == "." || id == ".." {
		return fmt.Errorf("provider id 非法: %q", id)
	}
	if !validID.MatchString(id) {
		return fmt.Errorf("provider id %q 含非法字符（仅允许字母、数字、. _ -）", id)
	}
	return nil
}

// Provider 是单个服务商配置。ID 是用户可见的短名（如 glm），Name 是展示名。
// Env 是要 export 的环境变量；值为 Keychain 占位时在 use 时解析为真值。
type Provider struct {
	ID   string            `json:"id"`
	Name string            `json:"name"`
	Env  map[string]string `json:"env,omitempty"`
}

// Config 是完整的配置文件内容。
type Config struct {
	Providers map[string]Provider `json:"providers"`
}

// ConfigPath 返回配置文件的绝对路径（~/.cc-select/providers.json）。
// 通过 CC_SELECT_CONFIG 环境变量可覆盖（便于测试）。
func ConfigPath() (string, error) {
	if p := os.Getenv("CC_SELECT_CONFIG"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cc-select", "providers.json"), nil
}

// UsedVars 返回所有 provider 的 env 中出现过的变量名集合（去重，顺序不稳定）。
// 用于切换时做"全量 unset"——见 docs/engineering-decisions.md §1。
//
// 运行时从配置派生，不持久化进 JSON，保证永远准确、无陈旧。
func (c *Config) UsedVars() []string {
	seen := map[string]struct{}{}
	for _, p := range c.Providers {
		for k := range p.Env {
			seen[k] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}

// ActiveVar 是记录"当前 shell 激活了哪个 provider"的环境变量名。
// use 命令导出它，current 命令读它。见 docs/engineering-decisions.md §3。
const ActiveVar = "CC_SELECT_ACTIVE"

// IsKeychainPlaceholder 判断一个 env 值是否为 Keychain 占位。
func IsKeychainPlaceholder(v string) bool {
	return len(v) > len(KeychainPlaceholderPrefix) &&
		v[:len(KeychainPlaceholderPrefix)] == KeychainPlaceholderPrefix
}

// KeychainService 从占位值中解析出 Keychain service 名。非占位返回空串。
func KeychainService(v string) string {
	if !IsKeychainPlaceholder(v) {
		return ""
	}
	return v[len(KeychainPlaceholderPrefix):]
}

// OfficialProviderID 是内置的官方 Claude provider 的 ID（env 为空 = 切换时 unset 一切）。
const OfficialProviderID = "claude-official"

// Default 返回一个带内置官方 provider 的空配置（首次运行/文件缺失时用）。
func Default() *Config {
	return &Config{
		Providers: map[string]Provider{
			OfficialProviderID: {
				ID:   OfficialProviderID,
				Name: "Claude 官方",
				Env:  nil,
			},
		},
	}
}
