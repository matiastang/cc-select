package secrets

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// ErrNotFound 表示密钥在 Keychain 中不存在。
// （go-keyring 用 keyring.ErrNotFound，这里包装为包内 sentinel 便于调用方判断。）
var ErrNotFound = errors.New("secret not found")

// KeyringStore 是基于系统 Keychain 的 SecretStore 实现。
// go-keyring 在 macOS 调 security 命令、Linux 走 dbus Secret Service、
// Windows 调 Credential Manager，全部纯 Go 无 CGO。
type KeyringStore struct{}

// New 返回系统 Keychain 后端。单例即可（无状态）。
func New() SecretStore { return KeyringStore{} }

// Get 从 Keychain 取密钥。不存在时返回 ErrNotFound。
func (KeyringStore) Get(service string) (string, error) {
	// go-keyring 的 account 字段这里固定用空串（service 已含完整标识）。
	v, err := keyring.Get(service, "")
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("keyring get %s: %w", service, err)
	}
	return v, nil
}

// Set 写入（覆盖）密钥到 Keychain。
func (KeyringStore) Set(service, value string) error {
	if err := keyring.Set(service, "", value); err != nil {
		return fmt.Errorf("keyring set %s: %w", service, err)
	}
	return nil
}

// Delete 删除密钥。不存在视为成功（幂等）。
func (KeyringStore) Delete(service string) error {
	err := keyring.Delete(service, "")
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil // 幂等
		}
		return fmt.Errorf("keyring delete %s: %w", service, err)
	}
	return nil
}
