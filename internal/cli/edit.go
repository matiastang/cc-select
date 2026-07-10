package cli

import (
	"bufio"
	"fmt"

	"github.com/cc-select/cc-select/internal/app"
	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/spf13/cobra"
)

var editFl addFlags

var editCmd = &cobra.Command{
	Use:  "edit <id>",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		id := args[0]
		old, exists := a.Config.Providers[id]
		if !exists {
			return fmt.Errorf(i18n.T("cli.edit.missing"), id)
		}

		// 旧 env 从 profile settings.json 读真值（providers.json 不再存 env）。
		oldEnv, _ := profile.ReadEnv(id)

		// flag 未显式覆盖的字段保留旧值；apiKey 留空 = 保持旧 token。
		fl := prefilledFrom(oldEnv, old, editFl)
		r := bufio.NewReader(cmd.InOrStdin())
		fl, err = readProviderInput(r, cmd.OutOrStdout(), fl, id, fl.preset)
		if err != nil {
			return err
		}
		if fl.apiKey == "" {
			fl.apiKey = oldEnv[string(old.AuthField)]
			if fl.apiKey == "" {
				fl.apiKey = oldEnv["ANTHROPIC_AUTH_TOKEN"]
			}
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

		// 处理 --add-field / --remove-field。
		fl = applyFieldEdits(oldEnv, fl)

		if err := upsertProvider(a, id, fl, providerMode); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cli.edit.updated", id))
		return nil
	},
}

func init() {
	localizeCmd(editCmd, "cli.edit.short", "cli.edit.long")
	rootCmd.AddCommand(editCmd)
	registerProviderFlags(editCmd, &editFl)
	editCmd.Flags().StringArrayVar(&editFl.addFields, "add-field", nil, "")
	editCmd.Flags().StringArrayVar(&editFl.removeFields, "remove-field", nil, "")
	localizeFlag(editCmd, "add-field", "cli.edit.addFieldFlag")
	localizeFlag(editCmd, "remove-field", "cli.edit.removeFieldFlag")
}

// prefilledFrom 用旧 env 真值 + 旧 provider 元信息填充 flag 默认。
// flag 未显式设置（空串）的字段用旧值；apiKey 不回填（留空=保持）。
func prefilledFrom(oldEnv map[string]string, old config.Provider, fl addFlags) addFlags {
	if fl.name == "" {
		fl.name = old.Name
	}
	if fl.baseURL == "" {
		fl.baseURL = oldEnv["ANTHROPIC_BASE_URL"]
	}
	if fl.model == "" {
		fl.model = oldEnv["ANTHROPIC_MODEL"]
	}
	if fl.preset == "" {
		fl.preset = old.PresetID
	}
	if fl.preset == "" {
		fl.preset = "custom"
	}
	if fl.apiFormat == "" {
		fl.apiFormat = old.APIFormat
	}
	if fl.authField == "" {
		fl.authField = old.AuthField
	}
	// 把旧 env 中没有出现在 flags 中的字段补进 --field，便于保留。
	overrides, _ := parseFieldFlags(fl.fields)
	for k, v := range oldEnv {
		if k == "ANTHROPIC_BASE_URL" || k == "ANTHROPIC_MODEL" {
			continue
		}
		if _, ok := overrides[k]; !ok {
			overrides[k] = v
		}
	}
	fl.fields = nil
	for k, v := range overrides {
		fl.fields = append(fl.fields, k+"="+v)
	}
	return fl
}

// applyFieldEdits 在旧 env 基础上应用 --add-field 与 --remove-field。
func applyFieldEdits(oldEnv map[string]string, fl addFlags) addFlags {
	adds, err := parseFieldFlags(fl.addFields)
	if err != nil {
		return fl
	}
	removes := map[string]struct{}{}
	for _, k := range fl.removeFields {
		removes[k] = struct{}{}
	}
	merged := map[string]string{}
	for k, v := range oldEnv {
		if _, ok := removes[k]; ok {
			continue
		}
		merged[k] = v
	}
	for k, v := range adds {
		merged[k] = v
	}
	// 把合并结果写回 fl.fields，供 upsertProvider 使用。
	overrides, err := parseFieldFlags(fl.fields)
	if err != nil {
		return fl
	}
	for k, v := range merged {
		overrides[k] = v
	}
	for k := range removes {
		delete(overrides, k)
	}
	fl.fields = nil
	for k, v := range overrides {
		fl.fields = append(fl.fields, k+"="+v)
	}
	return fl
}
