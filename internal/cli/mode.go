package cli

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/spf13/cobra"
)

// modeCmd 查看或设置「全局隔离模式」（写入 ~/.cc-select/prefs.json）。
// 机制与两种模式的区别见 docs/isolation-modes.md。
var modeCmd = &cobra.Command{
	Use:   "mode [settings-only|full]",
	Short: "查看或设置全局隔离模式",
	Long: `查看或设置 cc-select 的全局隔离模式（写入 ~/.cc-select/prefs.json）。

  cc-select mode                 查看当前全局模式
  cc-select mode settings-only   Mode B（默认）：仅 settings.json 隔离，其余内容共享
  cc-select mode full            Mode A：整目录隔离

单个 provider 可用  cc-select edit <id> --mode ...   做 per-provider 覆盖；
本次一次性切换可用  cc-select use  <id> --mode ...   （不落盘）。
优先级：一次性 > provider > 全局 > 默认(Mode B)。`,
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
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", shown)
			if pr.IsolationMode == "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "（未显式设置，使用默认 %s）\n", prefs.DefaultMode)
			}
			return nil
		}
		// 有参数 = 设置全局模式。
		m := prefs.Mode(args[0])
		if m != prefs.ModeSettingsOnly && m != prefs.ModeFull {
			return fmt.Errorf("无效模式 %q（可选：settings-only | full）", args[0])
		}
		pr.IsolationMode = m
		if err := prefs.Save(pr); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✓ 全局隔离模式设为 %s\n", m)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(modeCmd)
}
