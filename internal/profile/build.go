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
	"fmt"
	"os"
	"path/filepath"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/prefs"
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
			return "", nil, fmt.Errorf("provider %q 的 profile 缺失，请先 cc-select add %s", id, id)
		}
		env, err = ReadEnv(id)
		if err != nil {
			return "", nil, err
		}
	}
	if env == nil {
		env = map[string]string{}
	}

	switch mode {
	case prefs.ModeFull:
		return syncFull(id, env)
	default: // ModeSettingsOnly
		return syncSettingsOnly(id, env, &warnings)
	}
}

// syncFull 写 {"env": env} 并清理其余条目（权威隔离）。
func syncFull(id string, env map[string]string) (string, []string, error) {
	data, err := json.MarshalIndent(map[string]any{"env": env}, "", "  ")
	if err != nil {
		return "", nil, fmt.Errorf("序列化 env: %w", err)
	}
	dir, err := EnsureRaw(id, data) // 创建目录 + 原子写 settings.json
	if err != nil {
		return "", nil, err
	}
	if err := pruneNonSettings(dir); err != nil {
		return "", nil, fmt.Errorf("清理 profile 隔离: %w", err)
	}
	return dir, nil, nil
}

// syncSettingsOnly 写合并后的 settings.json + 链接共享 ~/.claude 其余条目。
func syncSettingsOnly(id string, env map[string]string, warnings *[]string) (string, []string, error) {
	home, err := ClaudeHome()
	if err != nil {
		return "", nil, fmt.Errorf("定位 ~/.claude: %w", err)
	}
	global, _ := os.ReadFile(filepath.Join(home, "settings.json"))

	merged, merr := mergeSettings(global, env)
	if merr != nil {
		// 全局 settings 不可解析 → 降级为仅 env（不阻断）。
		merged, _ = json.MarshalIndent(map[string]any{"env": env}, "", "  ")
		*warnings = append(*warnings, "读取/合并全局 settings.json 失败，已降级为仅 env: "+merr.Error())
	}

	dir, err := EnsureRaw(id, merged) // 创建目录 + 原子写合并后的 settings.json
	if err != nil {
		return "", nil, err
	}

	skipped, serr := shareEntries(dir, home, []string{denySettings})
	for _, s := range skipped {
		*warnings = append(*warnings, fmt.Sprintf("未共享 %s：%s", s.Name, s.Reason))
	}
	if serr != nil {
		*warnings = append(*warnings, "共享条目出错: "+serr.Error())
	}
	return dir, *warnings, nil
}
