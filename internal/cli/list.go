package cli

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/cc-select/cc-select/internal/app"
	"github.com/cc-select/cc-select/internal/config"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "列出所有已配置的 provider",
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

	fmt.Fprintf(w, "%-20s %-16s\n", "ID", "名称")
	fmt.Fprintln(w, "----------------------------------------")
	for _, id := range ids {
		p := cfg.Providers[id]
		marker := "  "
		if id == active {
			marker = "* " // 标当前
		}
		fmt.Fprintf(w, "%s%-20s %-16s\n", marker, p.ID, p.Name)
	}
	if active == "" {
		fmt.Fprintln(w, "\n（当前 shell 未激活任何 provider，运行 ccs use <id> 切换）")
	}
}
