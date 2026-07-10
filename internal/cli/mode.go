package cli

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/spf13/cobra"
)

// modeCmd 查看或设置「全局隔离模式」（写入 ~/.cc-select/prefs.json）。
// 机制与两种模式的区别见 docs/isolation-modes.md。
var modeCmd = &cobra.Command{
	Use:  "mode [settings-only|full]",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pr, err := prefs.Load()
		if err != nil {
			return err
		}
		// 无参数 = 打印当前全局模式。
		if len(args) == 0 {
			shown := pr.IsolationMode
			if shown == "" {
				shown = prefs.DefaultMode
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cli.mode.current", shown))
			if pr.IsolationMode == "" {
				fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("cli.mode.defaultHint", prefs.DefaultMode))
			}
			return nil
		}
		// 有参数 = 设置全局模式。
		m := prefs.Mode(args[0])
		if m != prefs.ModeSettingsOnly && m != prefs.ModeFull {
			return fmt.Errorf(i18n.T("cli.mode.invalid"), args[0])
		}
		pr.IsolationMode = m
		if err := prefs.Save(pr); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cli.mode.set", m))
		return nil
	},
}

func init() {
	localizeCmd(modeCmd, "cli.mode.short", "cli.mode.long")
	rootCmd.AddCommand(modeCmd)
}
