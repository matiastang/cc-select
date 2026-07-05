package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/cc-select/cc-select/internal/app"
	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/spf13/cobra"
)

// addFlags 收集 add/edit 命令的输入参数。
type addFlags struct {
	name    string
	baseURL string
	apiKey  string
	model   string
	mode    string
}

var addFl addFlags

var addCmd = &cobra.Command{
	Use:   "add <id>",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.New()
		if err != nil {
			return err
		}
		id := args[0]
		if err := config.ValidateID(id); err != nil {
			return err
		}
		if _, exists := a.Config.Providers[id]; exists {
			return fmt.Errorf(i18n.T("cli.add.exists"), id, id)
		}
		fl, err := readProviderInput(cmd.InOrStdin(), cmd.OutOrStdout(), addFl, id)
		if err != nil {
			return err
		}
		providerMode, err := normalizeProviderMode(fl.mode)
		if err != nil {
			return err
		}
		if err := upsertProvider(a, id, fl, providerMode); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cli.add.added", id))
		return nil
	},
}

func init() {
	localizeCmd(addCmd, "cli.add.short", "cli.add.long")
	rootCmd.AddCommand(addCmd)
	registerProviderFlags(addCmd, &addFl)
}

// registerProviderFlags 给 add/edit 命令注册共用的 provider 字段 flag。
func registerProviderFlags(c *cobra.Command, fl *addFlags) {
	c.Flags().StringVar(&fl.name, "name", "", "")
	c.Flags().StringVar(&fl.baseURL, "base-url", "", "")
	c.Flags().StringVar(&fl.apiKey, "api-key", "", "")
	c.Flags().StringVar(&fl.model, "model", "", "")
	c.Flags().StringVar(&fl.mode, "mode", "", "")
	localizeFlag(c, "name", "cli.add.nameFlag")
	localizeFlag(c, "base-url", "cli.add.baseURLFlag")
	localizeFlag(c, "api-key", "cli.add.apiKeyFlag")
	localizeFlag(c, "model", "cli.add.modelFlag")
	localizeFlag(c, "mode", "cli.add.modeFlag")
}

// normalizeProviderMode 把 --mode 的输入归一化为可存储的 per-provider 模式值。
//   - "" / "default" / "inherit" → ""（继承全局，即不设覆盖）；
//   - "settings-only" / "full" → 原样；
//   - 其他 → 报错。
//
// 注意：edit 命令的「未传 --mode = 保持旧值」语义由 edit 自行处理，不在此函数。
func normalizeProviderMode(raw string) (prefs.Mode, error) {
	switch raw {
	case "", "default", "inherit":
		return "", nil
	case string(prefs.ModeSettingsOnly), string(prefs.ModeFull):
		return prefs.Mode(raw), nil
	default:
		return "", fmt.Errorf(i18n.T("cli.add.modeInvalid"), raw)
	}
}

// readProviderInput 合并 flag 与交互式补全：flag 未提供的字段从 stdin 提示读取。
func readProviderInput(in io.Reader, out io.Writer, fl addFlags, id string) (addFlags, error) {
	// 默认 name 用 id。
	if fl.name == "" {
		fl.name = id
	}
	r := bufio.NewReader(in)
	prompt := func(label, cur string) (string, error) {
		if cur != "" {
			return cur, nil // flag 已提供，不交互
		}
		fmt.Fprintf(out, "%s: ", label)
		line, err := r.ReadString('\n')
		if err != nil && line == "" {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}
	var err error
	fl.baseURL, err = prompt(i18n.T("cli.add.prompts.baseURL"), fl.baseURL)
	if err != nil {
		return fl, err
	}
	fl.model, err = prompt(i18n.T("cli.add.prompts.model"), fl.model)
	if err != nil {
		return fl, err
	}
	// api-key 默认交互读取（不从 flag 暴露在 history）。留空则不设置。
	if fl.apiKey == "" {
		fmt.Fprint(out, i18n.T("cli.add.prompts.apiKey")+": ")
		line, _ := r.ReadString('\n')
		fl.apiKey = strings.TrimSpace(line)
	}
	return fl, nil
}

// upsertProvider 把输入组装成 provider：按解析后的隔离模式写入 profile（profile.Sync），
// providers.json 存 id/name + per-provider 模式覆盖。官方 id 不应走到这里（add 禁止、use 不建）。
// providerMode 是要持久化的 per-provider 覆盖（空=继承全局）。
func upsertProvider(a *app.App, id string, fl addFlags, providerMode prefs.Mode) error {
	env := map[string]string{}
	if fl.baseURL != "" {
		env["ANTHROPIC_BASE_URL"] = fl.baseURL
	}
	if fl.model != "" {
		env["ANTHROPIC_MODEL"] = fl.model
	}
	if fl.apiKey != "" {
		// token 明文进 profile settings.json（claude 读 settings.json 的 env）。
		env["ANTHROPIC_AUTH_TOKEN"] = fl.apiKey
	}
	if err := writeProvider(a, id, fl.name, env, providerMode); err != nil {
		return err
	}
	return config.Save(a.Config)
}

// writeProvider 按解析后的隔离模式构建 profile（profile.Sync），并把 id/name + 模式覆盖记入 providers.json。
// env 含明文敏感值（token）。供 add/edit 共用。
func writeProvider(a *app.App, id, name string, env map[string]string, providerMode prefs.Mode) error {
	// 实际生效模式 = per-provider 覆盖（若有）> 全局 > 默认。
	resolved := prefs.ResolveMode("", providerMode, a.Prefs.IsolationMode)
	if _, _, err := profile.Sync(id, env, resolved); err != nil {
		return fmt.Errorf(i18n.T("cli.add.profileWriteFailed"), err)
	}
	if name == "" {
		name = id
	}
	a.Config.Providers[id] = config.Provider{ID: id, Name: name, IsolationMode: providerMode}
	return nil
}
