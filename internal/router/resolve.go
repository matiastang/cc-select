// resolve.go 把 provider env 中的 keychain 占位解析为真值，带内存缓存（研究 D8）。
//
// 密钥真值只存在于 daemon 进程内存（解析后缓存），绝不落盘；
// 占位格式复用既有机制：$keychain:<service>（config.KeychainPlaceholderPrefix）。
package router

import (
	"fmt"
	"sync"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/secrets"
)

// cachedResolver 是带内存缓存的 EnvResolver 实现。
type cachedResolver struct {
	store secrets.SecretStore
	mu    sync.Mutex
	cache map[string]string // 占位值 → 真值
}

// NewKeychainResolver 构造解析器。store 生产用系统 keychain（secrets 包），
// 测试注入 FakeStore。
func NewKeychainResolver(store secrets.SecretStore) EnvResolver {
	return &cachedResolver{store: store, cache: map[string]string{}}
}

// Resolve：非占位原样返回；占位查 keychain（命中缓存直接返回）。
// 失败错误点名 service，便于用户定位是哪条密钥取不到。
func (c *cachedResolver) Resolve(v string) (string, error) {
	if !config.IsKeychainPlaceholder(v) {
		return v, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if got, ok := c.cache[v]; ok {
		return got, nil
	}
	svc := config.KeychainService(v)
	got, err := c.store.Get(svc)
	if err != nil {
		return "", fmt.Errorf("keychain get %q failed: %w", svc, err)
	}
	c.cache[v] = got
	return got, nil
}
