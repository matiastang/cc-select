package cli

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/app"
	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/cc-select/cc-select/internal/routes"
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

	// Mode P（proxy，specs/001）：第三方目标走专属分支（ensure daemon + SyncProxy +
	// 含路由同步语句的发射）。官方目标不参与 Mode P（研究 D2），落到下方既有路径。
	if mode == prefs.ModeProxy && target.ID != config.OfficialProviderID {
		return runUseProxy(cmd, target)
	}

	// 按模式（幂等）构建 profile：Mode B 重合并 settings + 自愈链接，Mode A 仅写 env。
	// 官方 provider 的 Sync 为 no-op。env=nil 表示沿用现有 profile 的 env（缺失则报错）。
	// proxy+官方：Sync 对官方 no-op，发射用 PlanProxy 官方回退（清伪 token）。
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
	if mode == prefs.ModeProxy {
		// 官方回退：unset CLAUDE_CONFIG_DIR + unset 伪 token（PlanProxy 官方分支）。
		tid, terr := routes.NewTID()
		if terr != nil {
			return terr
		}
		changes = switcher.PlanProxy(target, tid)
	}
	out := emitter.Emit(changes)

	// 语句走 stdout（供 eval），提示走 stderr（不污染 eval）。
	fmt.Fprint(cmd.OutOrStdout(), out)
	fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("cli.use.switched", target.ID, displayName(target)))
	return nil
}

// runUseProxy 是 Mode P 的 use 分支（specs/001 contracts/cli.md §3）。
//
// 发射关键点：路由同步语句（OpExec）在 eval 上下文里按 shell 的**真实** TID
// 执行 route switch——二进制无法得知 shell 里既有的 TID，守卫只负责首建；
// 因此本函数自身不写路由表（同步语句是唯一真值来源）。
func runUseProxy(cmd *cobra.Command, target config.Provider) error {
	// 先确保 daemon 在位（失败即中止并给恢复指引，FR-011）——profile 的
	// BASE_URL 要固化它的 addr，必须先拿到。
	addr, err := ensureRouterFn()
	if err != nil {
		return fmt.Errorf(i18n.T("cli.use.ensureRouterFailed"), err)
	}

	if _, warnings, serr := profile.SyncProxy(target.ID, "http://"+addr); serr != nil {
		return serr
	} else {
		for _, w := range warnings {
			fmt.Fprintf(cmd.ErrOrStderr(), i18n.T("cli.use.warningPrefix")+"%s\n", w)
		}
	}

	tid, err := routes.NewTID()
	if err != nil {
		return err
	}

	s := shell.Shell(useShellFlag)
	if s == shell.Unknown {
		s = shell.Detect()
	}
	emitter, err := shell.For(s)
	if err != nil {
		return err
	}

	changes := switcher.PlanProxy(target, tid)
	changes = append(changes, shell.Change{Op: shell.OpExec, Value: "route switch " + target.ID})
	out := emitter.Emit(changes)

	fmt.Fprint(cmd.OutOrStdout(), out)
	fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("cli.use.switched", target.ID, displayName(target)))
	return nil
}

// displayName returns provider 的展示名，空则回退 ID。
// 官方 provider 始终返回当前语言的翻译。
func displayName(p config.Provider) string {
	return p.DisplayName()
}
