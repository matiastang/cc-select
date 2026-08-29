package cli

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/routes"
	"github.com/spf13/cobra"
)

var currentCmd = &cobra.Command{
	Use: "current",
	RunE: func(cmd *cobra.Command, args []string) error {
		id := ""
		// Mode P（proxy）：终端身份存在且路由表有条目时，路由表是本终端真值——
		// CC_SELECT_ACTIVE 会滞后于会话内切换（specs/001 FR-008/US3）。
		if tid := envOrDefault(config.TerminalIDVar); tid != "" && routes.ValidateTID(tid) {
			if e, ok := routes.Get(tid); ok && e.Provider != "" {
				id = e.Provider
			}
		}
		if id == "" {
			id = envOrDefault(config.ActiveVar)
		}
		if id == "" {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cli.current.none"))
			return nil
		}
		// 尝试补全展示名（若该 provider 仍在配置中）。
		name := id
		if cfg, err := appLoadConfig(); err == nil {
			if p, ok := cfg.Providers[id]; ok {
				name = p.DisplayName()
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s（%s）\n", id, name)
		return nil
	},
}

func init() {
	localizeCmd(currentCmd, "cli.current.short", "cli.current.long")
	rootCmd.AddCommand(currentCmd)
}
