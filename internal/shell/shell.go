// Package shell 把"环境变量变更集"翻译成具体 shell 语法的语句串。
//
// 这是可扩展核心：shell.go 定义抽象（Change / Emitter / Shell），
// 各 shell 的实现（zsh.go、powershell.go 等）单独成文。
// 新增 shell 只需实现 Emitter 接口 + 加一个 For() 分支。
//
// 与 internal/switcher 解耦：switcher 只负责"算出要做什么变更"，
// shell 只负责"把变更写成某种 shell 能 eval 的语句"。这让切换的正确性
// （清理 + 导出）可在不启动真 shell 的情况下被纯单测覆盖。
package shell

import (
	"fmt"
	"strings"

	"github.com/cc-select/cc-select/internal/i18n"
)

// Op 描述单个变更的操作类型。
type Op int

const (
	OpSet   Op = iota // export NAME=VALUE
	OpUnset           // unset NAME
)

// Change 是单个环境变量变更。
type Change struct {
	Op    Op
	Name  string // 环境变量名
	Value string // 仅 OpSet 用
}

// Shell 是支持的 shell 类型。
type Shell string

const (
	Unknown    Shell = ""
	Zsh        Shell = "zsh"
	Bash       Shell = "bash"
	PowerShell Shell = "powershell"
)

// Emitter 把变更集翻译成某种 shell 语法的语句串。
type Emitter interface {
	// Emit 输出可直接被 eval / Invoke-Expression 执行的语句串。
	Emit(changes []Change) string
	// InitSnippet 输出 ccs() shell 函数体（由 cc-select init 注入）。
	// binaryPath 是 cc-select 可执行文件的绝对路径。
	InitSnippet(binaryPath string) string
}

// For 返回指定 shell 的 Emitter。未知 shell 返回错误。
func For(s Shell) (Emitter, error) {
	switch s {
	case Zsh, Bash: // zsh/bash 的 export/unset 语法兼容，共用 ZshEmitter
		return ZshEmitter{}, nil
	case PowerShell:
		return PowerShellEmitter{}, nil
	default:
		return nil, fmt.Errorf(i18n.T("errors.shell.unsupported"), s)
	}
}

// JoinChanges 把多个变更的 Emit 结果拼成一段语句串（实现复用）。
func JoinChanges(parts []string) string {
	return strings.Join(parts, "")
}
