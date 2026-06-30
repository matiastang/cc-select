package cli

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/app"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/spf13/cobra"
)

var editFl addFlags

var editCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "编辑一个 provider 的配置",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		id := args[0]
		old, exists := a.Config.Providers[id]
		if !exists {
			return fmt.Errorf("provider %q 不存在", id)
		}

		// 旧 env 从 profile settings.json 读真值（providers.json 不再存 env）。
		oldEnv, _ := profile.ReadEnv(id)

		// flag 未显式覆盖的字段保留旧值；apiKey 留空 = 保持旧 token。
		fl := prefilledFrom(oldEnv, old.Name, editFl)
		fl, err = readProviderInput(cmd.InOrStdin(), cmd.OutOrStdout(), fl, id)
		if err != nil {
			return err
		}
		if fl.apiKey == "" {
			fl.apiKey = oldEnv["ANTHROPIC_AUTH_TOKEN"] // 留空则保持
		}

		// 解析 per-provider 隔离模式：未传 --mode = 保持旧值；default/inherit = 清除覆盖；其余 = 设置。
		var providerMode prefs.Mode
		switch editFl.mode {
		case "":
			providerMode = old.IsolationMode
		case "default", "inherit":
			providerMode = ""
		default:
			pm, err := normalizeProviderMode(editFl.mode)
			if err != nil {
				return err
			}
			providerMode = pm
		}

		if err := upsertProvider(a, id, fl, providerMode); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✓ 已更新 provider %s\n", id)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(editCmd)
	registerProviderFlags(editCmd, &editFl)
}

// prefilledFrom 用旧 env 真值 + 旧 name 填充 flag 默认。
// flag 未显式设置（空串）的字段用旧值；显式传入的优先。apiKey 不回填（留空=保持）。
func prefilledFrom(oldEnv map[string]string, oldName string, fl addFlags) addFlags {
	if fl.name == "" {
		fl.name = oldName
	}
	if fl.baseURL == "" {
		fl.baseURL = oldEnv["ANTHROPIC_BASE_URL"]
	}
	if fl.model == "" {
		fl.model = oldEnv["ANTHROPIC_MODEL"]
	}
	return fl
}
