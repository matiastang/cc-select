// Package secrets 封装 API key 的安全存储。
//
// 默认走系统 Keychain（macOS Keychain / Linux Secret Service / Windows
// Credential Manager），由 zalando/go-keyring 提供跨平台实现（纯 Go，无 CGO）。
// 测试用 fake 实现注入。
//
// service 命名约定：cc-select:<providerID>:<envVar>，例：cc-select:glm:ANTHROPIC_AUTH_TOKEN。
package secrets

// SecretStore 抽象密钥存取。生产用 Keychain，测试用 Fake。
type SecretStore interface {
	// Get 取回密钥。不存在返回错误（实现层用 sentinel 或特定错误，见 keyring.go）。
	Get(service string) (string, error)
	// Set 写入（覆盖）密钥。
	Set(service, value string) error
	// Delete 删除密钥。不存在视为成功（幂等）。
	Delete(service string) error
}

// ServiceFor 按 cc-select 约定拼装 service 名。
func ServiceFor(providerID, envVar string) string {
	return "cc-select:" + providerID + ":" + envVar
}
