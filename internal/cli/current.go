package cli

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/spf13/cobra"
)

var currentCmd = &cobra.Command{
	Use:   "current",
	Short: "显示当前 shell 激活的 provider",
	Long: `显示当前 shell 激活的 provider。

只读 $CC_SELECT_ACTIVE 环境变量（反映"当前终端"状态），不查磁盘配置——
磁盘配置是全局共享的，不能代表某个特定 shell 的激活态。
详见 docs/engineering-decisions.md §3。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id := envOrDefault(config.ActiveVar)
		if id == "" {
			fmt.Fprintln(cmd.OutOrStdout(), "none（当前 shell 未激活任何 provider）")
			return nil
		}
		// 尝试补全展示名（若该 provider 仍在配置中）。
		name := id
		if cfg, err := appLoadConfig(); err == nil {
			if p, ok := cfg.Providers[id]; ok && p.Name != "" {
				name = p.Name
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s（%s）\n", id, name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(currentCmd)
}
