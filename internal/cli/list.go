package cli

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/cc-select/cc-select/internal/app"
	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		listProviders(cmd.OutOrStdout(), a.Config)
		return nil
	},
}

func init() {
	localizeCmd(listCmd, "cli.list.short", "cli.list.long")
	rootCmd.AddCommand(listCmd)
}

// listProviders 打印 provider 列表，标记当前 shell 激活项（读 $CC_SELECT_ACTIVE）。
func listProviders(w io.Writer, cfg *config.Config) {
	active := os.Getenv(config.ActiveVar)

	ids := make([]string, 0, len(cfg.Providers))
	for id := range cfg.Providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	fmt.Fprintf(w, "%-20s %-16s\n", i18n.T("cli.list.header.id"), i18n.T("cli.list.header.name"))
	fmt.Fprintln(w, "----------------------------------------")
	for _, id := range ids {
		p := cfg.Providers[id]
		marker := "  "
		if id == active {
			marker = "* "
		}
		fmt.Fprintf(w, "%s%-20s %-16s\n", marker, p.ID, p.DisplayName())
	}
	if active == "" {
		fmt.Fprintln(w, "\n"+i18n.T("cli.list.noActiveHint"))
	} else {
		// 尝试补全展示名（若该 provider 仍在配置中）。
		name := active
		if p, ok := cfg.Providers[active]; ok {
			name = p.DisplayName()
		}
		fmt.Fprintln(w, "\n"+i18n.T("cli.list.activeHint", name))
	}
}
