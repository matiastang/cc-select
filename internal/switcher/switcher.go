// Package switcher 把"切换到 provider X"翻译成环境变量变更（shell.Change）。
//
// 新机制（见 docs/engineering-decisions.md §6）：claude 优先读 settings.json 的 env、
// 覆盖 shell 变量，故 cc-select 不再 export ANTHROPIC_*，而是 export CLAUDE_CONFIG_DIR
// 指向 provider 的独立配置目录（internal/profile）。claude 读那份 settings.json。
//
//   - 普通 provider：set CLAUDE_CONFIG_DIR=<profile.Dir(id)> + set CC_SELECT_ACTIVE=id
//   - 官方 provider：unset CLAUDE_CONFIG_DIR（回默认 ~/.claude）+ set CC_SELECT_ACTIVE=id
//
// 与 internal/shell 解耦：这里只算"要做什么变更"，由 shell 包翻译成具体语法。
package switcher

import (
	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/cc-select/cc-select/internal/shell"
)

// Plan 计算切换到 target 所需的环境变量变更。
//
// 不再展开 ANTHROPIC_*（旧机制已被证伪），只操作 CLAUDE_CONFIG_DIR + CC_SELECT_ACTIVE。
// 官方 provider 产出 unset CLAUDE_CONFIG_DIR（让 claude 回默认配置目录）。
func Plan(target config.Provider) []shell.Change {
	var changes []shell.Change

	if target.ID == config.OfficialProviderID {
		// 官方：回默认 ~/.claude，unset 掉可能存在的 CLAUDE_CONFIG_DIR。
		changes = append(changes, shell.Change{Op: shell.OpUnset, Name: profile.ConfigVar})
	} else {
		// 普通：指向该 provider 的独立 profile 目录。
		// Dir 出错时无法切换——但 Plan 是纯逻辑无 error 返回，这里退化用空串。
		// 实际 use 命令会先用 profile.Exists 校验，调用 Plan 前 profile 已确保存在。
		dir, err := profile.Dir(target.ID)
		if err != nil {
			dir = ""
		}
		changes = append(changes, shell.Change{Op: shell.OpSet, Name: profile.ConfigVar, Value: dir})
	}

	// 记录当前激活的 provider（current 命令读它）。
	changes = append(changes, shell.Change{Op: shell.OpSet, Name: config.ActiveVar, Value: target.ID})

	return changes
}

// PlanProxy 计算 Mode P（proxy）切换到 target 所需的环境变量变更
// （specs/001 contracts/cli.md §3，研究 D5/D6）。
//
// 发射顺序有语义：TID 守卫先行（确保引用求值时身份已存在），随后
// AUTH_TOKEN 引用式指向 TID（伪 token）、CLAUDE_CONFIG_DIR 指向 profile
// （其 settings.json 的 env 仅含恒定的代理 BASE_URL，由 profile.SyncProxy 构造）。
//
// 官方 provider 不参与 Mode P（研究 D2）：回退官方语义（unset CLAUDE_CONFIG_DIR），
// 并额外 unset ANTHROPIC_AUTH_TOKEN——此前 proxy 模式注入的伪 token 不得发给官方端点。
func PlanProxy(target config.Provider, tid string) []shell.Change {
	if target.ID == config.OfficialProviderID {
		return []shell.Change{
			{Op: shell.OpUnset, Name: profile.ConfigVar},
			{Op: shell.OpUnset, Name: config.AuthTokenVar},
			{Op: shell.OpSet, Name: config.ActiveVar, Value: target.ID},
		}
	}

	// Dir 出错时退化空串：实际 use 命令在 Plan 前已用 profile.Exists 校验过。
	dir, err := profile.Dir(target.ID)
	if err != nil {
		dir = ""
	}
	return []shell.Change{
		{Op: shell.OpSetIfUnset, Name: config.TerminalIDVar, Value: tid},
		{Op: shell.OpSetRef, Name: config.AuthTokenVar, Value: config.TerminalIDVar},
		{Op: shell.OpSet, Name: profile.ConfigVar, Value: dir},
		{Op: shell.OpSet, Name: config.ActiveVar, Value: target.ID},
	}
}
