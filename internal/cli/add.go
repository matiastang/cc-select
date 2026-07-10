package cli

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/cc-select/cc-select/internal/app"
	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/cc-select/cc-select/internal/presets"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/spf13/cobra"
)

// addFlags 收集 add/edit 命令的输入参数。
type addFlags struct {
	name         string
	baseURL      string
	apiKey       string
	model        string
	mode         string
	preset       string
	apiFormat    string
	authField    string
	fields       []string
	addFields    []string // edit only
	removeFields []string // edit only
}

var addFl addFlags

var addCmd = &cobra.Command{
	Use:  "add <id>",
	Args: cobra.ExactArgs(1),
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
		r := bufio.NewReader(cmd.InOrStdin())
		presetID, err := resolvePreset(r, cmd.OutOrStdout(), addFl.preset)
		if err != nil {
			return err
		}
		fl, err := readProviderInput(r, cmd.OutOrStdout(), addFl, id, presetID)
		if err != nil {
			return err
		}
		fl.preset = presetID
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
	c.Flags().StringVar(&fl.preset, "preset", "", "")
	c.Flags().StringVar(&fl.apiFormat, "api-format", "", "")
	c.Flags().StringVar(&fl.authField, "auth-field", "", "")
	c.Flags().StringArrayVar(&fl.fields, "field", nil, "")
	localizeFlag(c, "name", "cli.add.nameFlag")
	localizeFlag(c, "base-url", "cli.add.baseURLFlag")
	localizeFlag(c, "api-key", "cli.add.apiKeyFlag")
	localizeFlag(c, "model", "cli.add.modelFlag")
	localizeFlag(c, "mode", "cli.add.modeFlag")
	localizeFlag(c, "preset", "cli.add.presetFlag")
	localizeFlag(c, "api-format", "cli.add.apiFormatFlag")
	localizeFlag(c, "auth-field", "cli.add.authFieldFlag")
	localizeFlag(c, "field", "cli.add.fieldFlag")
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

// resolvePreset 解析 --preset flag；若为空则进入交互式选择。
func resolvePreset(r *bufio.Reader, out io.Writer, presetFlag string) (string, error) {
	if presetFlag != "" {
		if _, ok := presets.ByID(presetFlag); !ok {
			return "", fmt.Errorf(i18n.T("cli.add.unknownPreset"), presetFlag)
		}
		return presetFlag, nil
	}
	return interactivePresetSelect(r, out)
}

// interactivePresetSelect 打印编号 preset 列表并读取用户选择。
func interactivePresetSelect(r *bufio.Reader, out io.Writer) (string, error) {
	byCategory := presets.AllByCategory()
	categories := presets.Categories()

	fmt.Fprintln(out, i18n.T("cli.add.presetPrompt"))
	idx := 1
	indexToID := map[int]string{}
	for _, cat := range categories {
		fmt.Fprintf(out, "\n[%s]\n", categoryDisplay(cat))
		for _, p := range byCategory[cat] {
			fmt.Fprintf(out, "  %d. %s\n", idx, p.DisplayName)
			indexToID[idx] = p.ID
			idx++
		}
	}
	fmt.Fprint(out, i18n.T("cli.add.presetChoice")+": ")
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	choice := strings.TrimSpace(line)
	if choice == "" {
		return "custom", nil
	}
	// 支持输入编号或 id。
	if n, ok := parseChoiceNumber(choice); ok {
		if id, ok := indexToID[n]; ok {
			return id, nil
		}
		return "", fmt.Errorf(i18n.T("cli.add.presetChoiceInvalid"), choice)
	}
	if _, ok := presets.ByID(choice); !ok {
		return "", fmt.Errorf(i18n.T("cli.add.unknownPreset"), choice)
	}
	return choice, nil
}

func parseChoiceNumber(s string) (int, bool) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err == nil && n > 0
}

func categoryDisplay(c presets.Category) string {
	key := "cli.add.category." + string(c)
	tr := i18n.T(key)
	if tr == key {
		return string(c)
	}
	return tr
}

// parseFieldFlags 把 --field KEY=VALUE 解析为 map。
func parseFieldFlags(flags []string) (map[string]string, error) {
	out := map[string]string{}
	for _, f := range flags {
		key, value, ok := strings.Cut(f, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf(i18n.T("cli.add.invalidFieldFlag"), f)
		}
		out[key] = value
	}
	return out, nil
}

// readProviderInput 合并 flag 与交互式补全：flag 未提供的字段从 stdin 提示读取。
// presetID 用于决定默认值和必填项。
func readProviderInput(r *bufio.Reader, out io.Writer, fl addFlags, id string, presetID string) (addFlags, error) {
	// 默认 name 用 id。
	if fl.name == "" {
		fl.name = id
	}
	p, _ := presets.ByID(presetID)

	prompt := func(label, cur, def string) (string, error) {
		if cur != "" {
			return cur, nil // flag 已提供，不交互
		}
		if def != "" {
			fmt.Fprintf(out, "%s [%s]: ", label, def)
		} else {
			fmt.Fprintf(out, "%s: ", label)
		}
		line, err := r.ReadString('\n')
		if err != nil {
			if def != "" && (err.Error() == "EOF" || err.Error() == "unexpected EOF") {
				fmt.Fprintln(out)
				return def, nil
			}
			return "", err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			return def, nil
		}
		return line, nil
	}

	fields, err := parseFieldFlags(fl.fields)
	if err != nil {
		return fl, err
	}
	overrides := make(map[string]string, len(fields))
	for k, v := range fields {
		overrides[k] = v
	}

	// baseURL / model 的覆盖值也可通过 --field 传入。
	if fl.baseURL != "" {
		overrides["ANTHROPIC_BASE_URL"] = fl.baseURL
	}
	if fl.model != "" {
		overrides["ANTHROPIC_MODEL"] = fl.model
	}
	if fl.apiFormat != "" {
		overrides["_api_format"] = fl.apiFormat
	}
	if fl.authField != "" {
		overrides["_auth_field"] = fl.authField
	}

	var err2 error
	fl.baseURL, err2 = prompt(i18n.T("cli.add.prompts.baseURL"), fl.baseURL, p.EnvTemplate["ANTHROPIC_BASE_URL"])
	if err2 != nil {
		return fl, err2
	}
	if fl.baseURL != "" {
		overrides["ANTHROPIC_BASE_URL"] = fl.baseURL
	}

	fl.model, err2 = prompt(i18n.T("cli.add.prompts.model"), fl.model, p.EnvTemplate["ANTHROPIC_MODEL"])
	if err2 != nil {
		return fl, err2
	}
	if fl.model != "" {
		overrides["ANTHROPIC_MODEL"] = fl.model
	}

	// API Key：非 OAuth preset 必填。
	if !p.OAuth {
		if fl.apiKey == "" {
			fmt.Fprint(out, i18n.T("cli.add.prompts.apiKey")+": ")
			line, _ := r.ReadString('\n')
			fl.apiKey = strings.TrimSpace(line)
		}
	}

	// 自定义占位符（如 AWS_REGION）。
	for _, key := range p.RequiredVars {
		if key == string(p.AuthField) {
			continue // 已由 apiKey 处理
		}
		if _, ok := overrides[key]; ok {
			continue
		}
		def := p.EnvTemplate[key]
		label := i18n.T("cli.add.prompts.customField", key)
		val, err := prompt(label, "", def)
		if err != nil {
			return fl, err
		}
		overrides[key] = val
	}

	fl.fields = nil
	for k, v := range overrides {
		fl.fields = append(fl.fields, k+"="+v)
	}
	sort.Strings(fl.fields)
	return fl, nil
}

// upsertProvider 把输入组装成 provider：按解析后的隔离模式写入 profile（profile.Sync），
// providers.json 存 id/name + per-provider 模式覆盖。官方 id 不应走到这里（add 禁止、use 不建）。
// providerMode 是要持久化的 per-provider 覆盖（空=继承全局）。
func upsertProvider(a *app.App, id string, fl addFlags, providerMode prefs.Mode) error {
	overrides, err := parseFieldFlags(fl.fields)
	if err != nil {
		return err
	}
	// 命令行显式传入的 --base-url / --model 优先级最高。
	if fl.baseURL != "" {
		overrides["ANTHROPIC_BASE_URL"] = fl.baseURL
	}
	if fl.model != "" {
		overrides["ANTHROPIC_MODEL"] = fl.model
	}
	env, missing, err := presets.BuildEnv(fl.preset, fl.apiKey, overrides)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return fmt.Errorf(i18n.T("cli.add.requiredMissing"), id, presets.FormatMissing(missing))
	}
	if err := writeProvider(a, id, fl.name, env, providerMode, fl.preset, fl.apiFormat, fl.authField); err != nil {
		return err
	}
	return config.Save(a.Config)
}

// writeProvider 按解析后的隔离模式构建 profile（profile.Sync），并把 id/name + 模式覆盖 + preset 元数据记入 providers.json。
// env 含明文敏感值（token）。供 add/edit 共用。
func writeProvider(a *app.App, id, name string, env map[string]string, providerMode prefs.Mode, presetID, apiFormat, authField string) error {
	// 实际生效模式 = per-provider 覆盖（若有）> 全局 > 默认。
	resolved := prefs.ResolveMode("", providerMode, a.Prefs.IsolationMode)
	if _, _, err := profile.Sync(id, env, resolved); err != nil {
		return fmt.Errorf(i18n.T("cli.add.profileWriteFailed"), err)
	}
	if name == "" {
		name = id
	}
	a.Config.Providers[id] = config.Provider{
		ID:            id,
		Name:          name,
		IsolationMode: providerMode,
		PresetID:      presetID,
		APIFormat:     apiFormat,
		AuthField:     authField,
	}
	return nil
}
