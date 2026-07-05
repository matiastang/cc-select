package cli

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/app"
	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/spf13/cobra"
)

var removeForce bool

var removeCmd = &cobra.Command{
	Use:     "remove <id>",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		id := args[0]

		// 官方 provider 不允许删除（它是回退项，且 use 依赖它）。
		if id == config.OfficialProviderID {
			return fmt.Errorf(i18n.T("cli.remove.official"), id)
		}
		if _, exists := a.Config.Providers[id]; !exists {
			return fmt.Errorf(i18n.T("cli.remove.missing"), id)
		}

		// 删除该 provider 的 profile 目录（含 settings.json）。
		if err := profile.Remove(id); err != nil {
			return fmt.Errorf(i18n.T("cli.remove.profileFailed"), err)
		}

		delete(a.Config.Providers, id)
		if err := config.Save(a.Config); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cli.remove.deleted", id))
		return nil
	},
}

func init() {
	localizeCmd(removeCmd, "cli.remove.short", "cli.remove.long")
	rootCmd.AddCommand(removeCmd)
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "")
	localizeFlag(removeCmd, "force", "cli.remove.forceFlag")
}
