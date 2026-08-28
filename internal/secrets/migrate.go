// migrate.go 把 provider env 中的敏感明文值迁入 keychain（specs/001 研究 D8 / US4）。
//
// 启用 Mode P 时调用：ANTHROPIC_AUTH_TOKEN / ANTHROPIC_API_KEY 的明文写入
// keychain（service 按既有约定 cc-select:<id>:<var>），providers.json 原值替换为
// $keychain: 占位——daemon 侧解析。幂等（已是占位跳过）；单条失败不中断其余，
// 失败项保持明文（不产生半迁移态）。离开 Mode P 不回迁。
package secrets

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/config"
)

// sensitiveVars 是需要迁入 keychain 的敏感 env 变量名。
var sensitiveVars = []string{"ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY"}

// MigrateAll 迁移 cfg 中全部 provider 的敏感值（就地修改 cfg），返回
// （迁移条数, 失败明细）。调用方负责把修改后的 cfg 持久化（config.Save）。
func MigrateAll(store SecretStore, cfg *config.Config) (migrated int, failed []string) {
	for id, p := range cfg.Providers {
		if p.Env == nil {
			continue
		}
		n, f := MigrateEnv(store, id, p.Env)
		migrated += n
		failed = append(failed, f...)
		cfg.Providers[id] = p // range 得到副本，写回
	}
	return migrated, failed
}

// MigrateEnv 迁移单个 provider env 的敏感值（就地修改；保存路径在钥匙串开关
// 开启时调用）。单条失败保持明文不中断，失败明细点名条目。
func MigrateEnv(store SecretStore, providerID string, env map[string]string) (migrated int, failed []string) {
	for _, v := range sensitiveVars {
		val, ok := env[v]
		if !ok || val == "" || config.IsKeychainPlaceholder(val) {
			continue
		}
		svc := ServiceFor(providerID, v)
		if err := store.Set(svc, val); err != nil {
			failed = append(failed, fmt.Sprintf("%s:%s: %v", providerID, v, err))
			continue
		}
		env[v] = config.KeychainPlaceholderPrefix + svc
		migrated++
	}
	return migrated, failed
}
