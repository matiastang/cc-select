package profile

import (
	"os"
	"path/filepath"
)

// ClaudeHome 返回 claude 的默认配置家目录（~/.claude）。
// Mode B 把这里的条目链接进 profile 目录以共享。
//
// 可用 CC_SELECT_CLAUDE_HOME 覆盖（便于测试）。注意：这里定位的是 claude 的家，
// 与 cc-select 的 CLAUDE_CONFIG_DIR（指向 profile 目录）是两回事——前者是「被共享的源」。
func ClaudeHome() (string, error) {
	if p := os.Getenv("CC_SELECT_CLAUDE_HOME"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

// entry 是一个需要共享的 ~/.claude 条目声明。
type entry struct {
	name  string
	isDir bool
	// targetRel 是链接目标相对 claudeHome 的路径；空 = 默认 <claudeHome>/<name>。
	// 用于 home 根的 sibling 文件——例如 .claude.json 实际在 ~/.claude.json
	// （home 根），而非 ~/.claude/.claude.json，故用 "../.claude.json" 指向 sibling。
	targetRel string
}

// sharedEntries 是 Mode B 需要共享（且目录型需预创建）的已知 claude 状态条目白名单。
// 这是 Mode B 唯一的「跟随 claude 版本」维护点——claude 新增状态目录时按需补充。
//
// claude 经 $CLAUDE_CONFIG_DIR 写「目录」时目标必须存在（否则 mkdir 穿悬挂链接失败），
// 故目录型条目需先在 ~/.claude 建好再链接。文件型可悬挂（写入穿透创建）。
//
// .claude.json 必须共享：它是 claude 启动时读取的主配置/状态文件（OAuth 账号、项目历史等），
// 缺失会直接报 "Claude configuration file not found"。它在 home 根（~/.claude.json）而非
// ~/.claude/ 内，故用 targetRel 指向 sibling。实测确认 CLAUDE_CONFIG_DIR 会重定位它。
var sharedEntries = []entry{
	{"projects", true, ""},
	{"todos", true, ""},
	{"shell-snapshots", true, ""},
	{"statsig", true, ""},
	{"ide", true, ""},
	{"plugins", true, ""},
	{"commands", true, ""},
	{"agents", true, ""},
	{"skills", true, ""},
	{"output-styles", true, ""},
	{"history.json", false, ""},
	{".mcp.json", false, ""},
	// home 根 sibling：CLAUDE_CONFIG_DIR 重定位后 profile 里也会有一份，需软链回 home 根共享。
	{".claude.json", false, "../.claude.json"},
}

// denySettings 是永远不共享、恒为 profile 真实文件的条目。
const denySettings = "settings.json"
