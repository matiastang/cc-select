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
	"github.com/cc-select/cc-select/internal/prefs"
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

// SyncProfile 是 add/edit/web 保存路径的统一 profile 构造入口：
// proxy 模式转 SyncProxy（addr 来自 ResolveAddr——无需 daemon 在位，只是写指向），
// 其余模式直通 profile.Sync。消除「全局 proxy 下保存被 Sync 防误用守卫拦截」的死角
// （真机使用中发现）。官方 provider 两边都是 no-op。
func SyncProfile(id string, env map[string]string, mode prefs.Mode) (dir string, warnings []string, err error) {
	if mode == prefs.ModeProxy && id != config.OfficialProviderID {
		addr, aerr := ResolveAddr()
		if aerr != nil {
			return "", nil, aerr
		}
		return profile.SyncProxy(id, "http://"+addr)
	}
	return profile.Sync(id, env, mode)
}
