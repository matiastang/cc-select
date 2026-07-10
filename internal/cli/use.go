package cli

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/app"
	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/cc-select/cc-select/internal/shell"
	"github.com/cc-select/cc-select/internal/switcher"
	"github.com/spf13/cobra"
)

var useShellFlag string
var useModeFlag string

var useCmd = &cobra.Command{
	Use:  "use <provider>",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUse(cmd, args)
	},
}

func init() {
	localizeCmd(useCmd, "cli.use.short", "cli.use.long")
	rootCmd.AddCommand(useCmd)
	useCmd.Flags().StringVar(&useShellFlag, "shell", "", "")
	useCmd.Flags().StringVar(&useModeFlag, "mode", "", "")
	localizeFlag(useCmd, "shell", "cli.use.shellFlag")
	localizeFlag(useCmd, "mode", "cli.use.modeFlag")
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
			fmt.Fprintf(cmd.ErrOrStderr(), i18n.T("cli.use.warningPrefix")+"%s\n", w)
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
	fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("cli.use.switched", target.ID, displayName(target)))
	return nil
}

// displayName returns provider 的展示名，空则回退 ID。
// 官方 provider 始终返回当前语言的翻译。
func displayName(p config.Provider) string {
	return p.DisplayName()
}
