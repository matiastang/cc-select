// build.go 是 profile 目录的统一构造入口 Sync——add/edit/use/web 共用。
//
// 它按隔离模式（prefs.Mode）把某 provider 的 profile 目录构建到「正确状态」（幂等）：
//
//   - ModeFull（A）：写 {"env": env} 并清理其余条目 → 真隔离（= 改动前 Ensure 行为）。
//   - ModeSettingsOnly（B）：写 mergeSettings(全局 settings.json, env)，并把 ~/.claude 的
//     其余条目链接进 profile 目录共享；每次调用自愈（settings 重合并、链接修复）。
//
// env == nil 表示「沿用现有 profile 的 env」（use 路径）；非 nil 表示 add/edit 传入的新 env。
// 返回 (dir, warnings, err)：warnings 是非致命提示（如个别条目未共享、非空真实条目被跳过），
// 供调用方告警；不阻断切换。
package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/cc-select/cc-select/internal/secrets"
)

// Sync 按 mode 把 provider 的 profile 目录构建到正确状态（幂等）。
// 官方 provider 返回 ("", nil, nil)（无 profile）。
func Sync(id string, env map[string]string, mode prefs.Mode) (dir string, warnings []string, err error) {
	if id == config.OfficialProviderID {
		return "", nil, nil
	}
	if !mode.Valid() {
		mode = prefs.DefaultMode
	}

	// env == nil：use 路径，沿用现有 profile 的 env。
	if env == nil {
		exists, eerr := Exists(id)
		if eerr != nil {
			return "", nil, eerr
		}
		if !exists {
			return "", nil, fmt.Errorf(i18n.T("errors.provider.missingProfile"), id, id)
		}
		env, err = ReadEnv(id)
		if err != nil {
			return "", nil, err
		}
	}
	if env == nil {
		env = map[string]string{}
	}

	// 非 proxy 模式：claude 直读 profile settings.json，$keychain: 占位对它无意义，
	// 写入前解析为真值（Mode P→B 往返可用性的必要环节；失败降级保留占位并告警）。
	env, warns := resolvePlaceholders(env)
	warnings = append(warnings, warns...)

	switch mode {
	case prefs.ModeFull:
		return syncFull(id, env)
	case prefs.ModeProxy:
		// Mode P 的 env 真值是「路由 daemon 地址」而非 provider env，Sync 拿不到它。
		// 防误用：明确报错引导调用方走 SyncProxy（use 命令的 proxy 分支）。
		return "", nil, errors.New(i18n.T("errors.profile.proxyNeedsSyncProxy"))
	default: // ModeSettingsOnly
		return syncSettingsOnly(id, env, &warnings)
	}
}

// SyncProxy 按 Mode P（proxy）构造 profile（幂等自愈）。
//
// 与 Mode B 的唯一差别是 env 的真值：settings.json 的 env 整体替换为**仅含
// 恒定的 ANTHROPIC_BASE_URL=routerBaseURL**（specs/001 研究 D5）——
//   - 静态项（BASE_URL）进 settings.json，借整体替换语义屏蔽全局 env 污染；
//   - 动态项（伪 token=CC_SELECT_TID）经 shell 注入，绝不落 profile；
//   - provider 真实 env（含密钥）只存于 providers.json/keychain，由 daemon 使用。
//
// 其余行为（全局非 env 字段合并、共享链接自愈）与 Mode B 完全一致。
// 官方 provider 返回 ("", nil, nil)（无 profile，Mode P 不适用官方，D2）。
func SyncProxy(id string, routerBaseURL string) (dir string, warnings []string, err error) {
	if id == config.OfficialProviderID {
		return "", nil, nil
	}
	// 上抬真值（评审 #1）：本函数即将把 profile env 覆写为仅含代理 BASE_URL；
	// 若 providers.json 尚无 env 而 profile 里存着真实 provider env（legacy 形态），
	// 先抄进 providers.json——否则切回 Mode A/B 时真值永久丢失（往返污染）。
	upliftLegacyEnv(id, routerBaseURL)
	env := map[string]string{"ANTHROPIC_BASE_URL": routerBaseURL}
	return syncSettingsOnly(id, env, &warnings)
}

// upliftLegacyEnv 把仅存在于 profile settings.json 的 provider env 上抬进
// providers.json（幂等：providers.json 已有 env 则跳过；profile env 是上一次
// SyncProxy 的代理覆盖产物也跳过——其 BASE_URL 等于 routerBaseURL）。
func upliftLegacyEnv(id, routerBaseURL string) {
	cfg, err := config.Load()
	if err != nil {
		return // 读不到就尽力而为（主流程不因上抬失败中断）
	}
	p, ok := cfg.Providers[id]
	if !ok || len(p.Env) > 0 {
		return
	}
	env, err := ReadEnv(id)
	if err != nil || len(env) == 0 {
		return
	}
	if env["ANTHROPIC_BASE_URL"] == routerBaseURL {
		return // 代理覆盖产物，无真值可救
	}
	p.Env = env
	cfg.Providers[id] = p
	_ = config.Save(cfg)
}

// syncFull 写 {"env": env} 并清理其余条目（权威隔离）。
func syncFull(id string, env map[string]string) (string, []string, error) {
	data, err := json.MarshalIndent(map[string]any{"env": env}, "", "  ")
	if err != nil {
		return "", nil, fmt.Errorf(i18n.T("profile.serializeEnv"), err)
	}
	dir, err := EnsureRaw(id, data) // 创建目录 + 原子写 settings.json
	if err != nil {
		return "", nil, err
	}
	if err := pruneNonSettings(dir); err != nil {
		return "", nil, fmt.Errorf(i18n.T("profile.cleanIsolation"), err)
	}
	return dir, nil, nil
}

// syncSettingsOnly 写合并后的 settings.json + 链接共享 ~/.claude 其余条目。
func syncSettingsOnly(id string, env map[string]string, warnings *[]string) (string, []string, error) {
	home, err := ClaudeHome()
	if err != nil {
		return "", nil, fmt.Errorf(i18n.T("profile.locateClaudeHome"), err)
	}
	global, _ := os.ReadFile(filepath.Join(home, "settings.json"))

	merged, merr := mergeSettings(global, env)
	if merr != nil {
		// 全局 settings 不可解析 → 降级为仅 env（不阻断）。
		merged, _ = json.MarshalIndent(map[string]any{"env": env}, "", "  ")
		*warnings = append(*warnings, i18n.T("profile.mergeGlobalSettingsFailed")+merr.Error())
	}

	dir, err := EnsureRaw(id, merged) // 创建目录 + 原子写合并后的 settings.json
	if err != nil {
		return "", nil, err
	}

	skipped, serr := shareEntries(dir, home, []string{denySettings})
	for _, s := range skipped {
		*warnings = append(*warnings, fmt.Sprintf(i18n.T("profile.notShared"), s.Name, s.Reason))
	}
	if serr != nil {
		*warnings = append(*warnings, i18n.T("profile.shareError", serr.Error()))
	}
	return dir, *warnings, nil
}

// SecretGetter 从 keychain 取真值（service 全名）。包级变量供测试注入 FakeStore；
// 生产默认走系统 keychain（secrets 包，go-keyring 纯 Go 无 CGO）。
var SecretGetter func(service string) (string, error) = defaultSecretGetter

func defaultSecretGetter(service string) (string, error) {
	return secrets.New().Get(service)
}

// resolvePlaceholders 把 env 中的 $keychain: 占位解析为真值。
// 单条失败不中断：保留占位并产出 warning（claude 会因无效 token 明确报错，
// 好过静默丢字段）。env 为 nil 时原样返回。
func resolvePlaceholders(env map[string]string) (map[string]string, []string) {
	var warns []string
	out := make(map[string]string, len(env))
	for k, v := range env {
		if !config.IsKeychainPlaceholder(v) {
			out[k] = v
			continue
		}
		real, err := SecretGetter(config.KeychainService(v))
		if err != nil {
			warns = append(warns, fmt.Sprintf("%s: keychain get failed: %v", k, err))
			out[k] = v
			continue
		}
		out[k] = real
	}
	return out, warns
}
