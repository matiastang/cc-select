package cli

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/app"
	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/spf13/cobra"
)

var removeForce bool

var removeCmd = &cobra.Command{
	Use:     "remove <id>",
	Aliases: []string{"rm"},
	Short:   "删除一个 provider",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		id := args[0]

		// 官方 provider 不允许删除（它是回退项，且 use 依赖它）。
		if id == config.OfficialProviderID {
			return fmt.Errorf("内置 provider %s 不可删除", id)
		}
		if _, exists := a.Config.Providers[id]; !exists {
			return fmt.Errorf("provider %q 不存在", id)
		}

		// 删除该 provider 的 profile 目录（含 settings.json）。
		if err := profile.Remove(id); err != nil {
			return fmt.Errorf("删除 profile: %w", err)
		}

		delete(a.Config.Providers, id)
		if err := config.Save(a.Config); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✓ 已删除 provider %s\n", id)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "跳过确认（保留位，当前实现无交互确认）")
}
