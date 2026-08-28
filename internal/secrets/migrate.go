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
		for _, v := range sensitiveVars {
			val, ok := p.Env[v]
			if !ok || val == "" || config.IsKeychainPlaceholder(val) {
				continue
			}
			svc := ServiceFor(id, v)
			if err := store.Set(svc, val); err != nil {
				failed = append(failed, fmt.Sprintf("%s:%s: %v", id, v, err))
				continue
			}
			p.Env[v] = config.KeychainPlaceholderPrefix + svc
			migrated++
		}
		cfg.Providers[id] = p // range 得到副本，写回
	}
	return migrated, failed
}
