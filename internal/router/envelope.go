// envelope.go 提供 daemon 侧的 provider env 统一取值。
//
// 两代存储并存（评审 #1 的姊妹问题）：新形态真值在 providers.json（Mode P 迁移/
// 上抬后的权威源）；legacy 形态（早期 GUI 原文编辑时代创建的 provider）真值只在
// profile settings.json。daemon 按「providers.json 优先，空则回退 profile」取值，
// 保证 route switch 到一个从未 use 过的 legacy provider 也能正确转发。
package router

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/profile"
)

// providerEnvFor 返回 provider 的 env map。
// provider 不存在 → error；env 为空（两处都无）→ 空 map（调用方按缺配置报错）。
func providerEnvFor(providerID string) (map[string]string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	p, ok := cfg.Providers[providerID]
	if !ok {
		return nil, fmt.Errorf("provider %q not in providers.json", providerID)
	}
	if len(p.Env) > 0 {
		return p.Env, nil
	}
	return profile.ReadEnv(providerID) // legacy 回退；读取失败或为空都交给调用方判定
}
