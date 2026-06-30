package cli

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/app"
	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/cc-select/cc-select/internal/shell"
	"github.com/cc-select/cc-select/internal/switcher"
	"github.com/spf13/cobra"
)

var useShellFlag string
var useModeFlag string

var useCmd = &cobra.Command{
	Use:   "use <provider>",
	Short: "切换当前 shell 到指定 provider（输出供 eval 的 shell 语句）",
	Long: `切换当前终端到指定 provider。

机制：ccs use X 导出 CLAUDE_CONFIG_DIR 指向 X 的独立配置目录，
claude 启动时读该目录的 settings.json（含 X 的 env），从而走 X 服务商。
官方 provider（claude-official）则 unset CLAUDE_CONFIG_DIR，回默认 ~/.claude。

直接调用 cc-select use <provider> 只会打印语句；要生效需：
  eval "$(cc-select use <provider>)"
或通过别名：ccs use <provider>`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUse(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(useCmd)
	useCmd.Flags().StringVar(&useShellFlag, "shell", "",
		"目标 shell 语法（zsh/bash/powershell），默认自动检测")
	useCmd.Flags().StringVar(&useModeFlag, "mode", "",
		"一次性隔离模式（settings-only|full），仅本次、不落盘")
}

func runUse(cmd *cobra.Command, args []string) error {
	a, err := app.New()
	if err != nil {
		return err
	}

	target, err := a.Config.Provider(args[0])
	if err != nil {
		return err
	}

	// 解析最终隔离模式：一次性 --mode > provider 覆盖 > 全局 > 默认(Mode B)。
	mode := prefs.ResolveMode(prefs.Mode(useModeFlag), target.IsolationMode, a.Prefs.IsolationMode)

	// 按模式（幂等）构建 profile：Mode B 重合并 settings + 自愈链接，Mode A 仅写 env。
	// 官方 provider 的 Sync 为 no-op。env=nil 表示沿用现有 profile 的 env（缺失则报错）。
	if _, warnings, serr := profile.Sync(target.ID, nil, mode); serr != nil {
		return serr
	} else {
		for _, w := range warnings {
			fmt.Fprintf(cmd.ErrOrStderr(), "⚠ %s\n", w)
		}
	}

	// 解析目标 shell 语法。
	s := shell.Shell(useShellFlag)
	if s == shell.Unknown {
		s = shell.Detect()
	}
	emitter, err := shell.For(s)
	if err != nil {
		return err
	}

	changes := switcher.Plan(target)
	out := emitter.Emit(changes)

	// 语句走 stdout（供 eval），提示走 stderr（不污染 eval）。
	fmt.Fprint(cmd.OutOrStdout(), out)
	fmt.Fprintf(cmd.ErrOrStderr(), "→ 已切换到 %s（%s）。新终端将继承此环境。\n",
		target.ID, displayName(target))
	return nil
}

// displayName 返回 provider 的展示名，空则回退 ID。
func displayName(p config.Provider) string {
	if p.Name != "" {
		return p.Name
	}
	return p.ID
}
