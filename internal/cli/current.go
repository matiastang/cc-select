package cli

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/spf13/cobra"
)

var currentCmd = &cobra.Command{
	Use: "current",
	RunE: func(cmd *cobra.Command, args []string) error {
		id := envOrDefault(config.ActiveVar)
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
