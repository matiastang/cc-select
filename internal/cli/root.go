// Package cli 是 cc-select 命令行的唯一组装点。
// 它用 cobra 把各子命令挂到 rootCmd 上，业务逻辑下沉到 internal 下其他包。
package cli

import (
	"fmt"
	"os"

	"github.com/cc-select/cc-select/internal/version"
	"github.com/spf13/cobra"
)

// rootCmd 是 cc-select 的顶层命令。
var rootCmd = &cobra.Command{
	Use:   "cc-select",
	Short: "Shell-level AI provider isolation for Claude Code",
	Long: `cc-select 让同一台机器上的不同终端窗口使用不同的 AI 模型服务商。

与 cc-switch（全局切换）不同，cc-select 在当前 shell 内 export 环境变量，
只影响当前终端及其子进程，实现 shell 级（per-terminal）隔离。

切换通过 ccs 别名（由 cc-select init 注入）完成：ccs use <provider>`,
	SilenceUsage: true,
}

// Execute 解析参数并执行对应子命令，返回进程退出码。
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		// cobra 默认会把 usage 打到 stderr，SilenceUsage 已抑制非预期 usage。
		// 错误信息由 cobra 自身打印，这里仅返回非零退出码。
		return 1
	}
	return 0
}

func init() {
	rootCmd.Version = version.Version
	// cobra 默认会把 version 子命令的模板做格式化，这里保持简洁。
	rootCmd.SetVersionTemplate(fmt.Sprintf("cc-select %s\n", version.Version))
	// 让退出码可被 os.Exit 捕获（cobra 默认 panic 行为不适用）。
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
}
