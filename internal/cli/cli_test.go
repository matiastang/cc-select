package cli

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/app"
	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/cc-select/cc-select/internal/secrets"
	"github.com/spf13/cobra"
)

// setTempCfg 把 CC_SELECT_CONFIG 指向临时目录下的 providers.json，
// 使 config 与 profile 都落在 tempdir（隔离、无副作用）。
// 同时把 CC_SELECT_CLAUDE_HOME 指向另一个空 tempdir——Mode B 的 Sync 会读 ~/.claude，
// 这样可避免测试污染真实 claude 环境、也不向真实目录建链接。
func setTempCfg(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CC_SELECT_CONFIG", dir+"/providers.json")
	t.Setenv("CC_SELECT_CLAUDE_HOME", t.TempDir())
	// 测试默认使用英文输出，避免受运行环境语言影响。
	t.Setenv("CC_SELECT_LANGUAGE", "en")
}

// resetFlags 清掉 add/edit/use/init/remove 的全局 flag 变量，
// 避免上一次命令解析的值泄漏到下一次（cobra 把值写回这些全局变量）。
func resetFlags() {
	addFl = addFlags{}
	editFl = addFlags{}
	useShellFlag = ""
	useModeFlag = ""
	initShellFlag = ""
	removeForce = false
}

// execRoot 通过 rootCmd 执行一条子命令，捕获 stdout/stderr。
func execRoot(t *testing.T, stdin string, args ...string) (out, errOut string, err error) {
	t.Helper()
	resetFlags()
	var ob, eb bytes.Buffer
	rootCmd.SetOut(&ob)
	rootCmd.SetErr(&eb)
	rootCmd.SetIn(strings.NewReader(stdin))
	rootCmd.SetArgs(args)
	err = rootCmd.Execute()
	return ob.String(), eb.String(), err
}

// newCmd 返回一个带缓冲 out/err 的裸 cobra.Command，用于直接调用 runXxx 函数。
func newCmd() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	c := &cobra.Command{}
	var ob, eb bytes.Buffer
	c.SetOut(&ob)
	c.SetErr(&eb)
	return c, &ob, &eb
}

// ---- 纯函数 / 小工具 ----

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("CC_SELECT_TEST_VAR", "hello")
	if got := envOrDefault("CC_SELECT_TEST_VAR"); got != "hello" {
		t.Errorf("已设置 want hello got %q", got)
	}
	if got := envOrDefault("CC_SELECT_DEFINITELY_UNSET_VAR"); got != "" {
		t.Errorf("未设置应返回空串 got %q", got)
	}
}

func TestDisplayName(t *testing.T) {
	if got := displayName(config.Provider{ID: "glm", Name: "智谱"}); got != "智谱" {
		t.Errorf("有 Name 应返回 Name，got %q", got)
	}
	if got := displayName(config.Provider{ID: "glm"}); got != "glm" {
		t.Errorf("无 Name 应回退 ID，got %q", got)
	}
}

func TestListProviders(t *testing.T) {
	t.Setenv(config.ActiveVar, "glm")
	cfg := &config.Config{Providers: map[string]config.Provider{
		"glm":      {ID: "glm", Name: "GLM", Env: map[string]string{"A": "1", "B": "2"}},
		"deepseek": {ID: "deepseek", Name: "DS"},
	}}
	var b bytes.Buffer
	listProviders(&b, cfg)
	out := b.String()
	// 激活项应有 "* " 标记。
	if !strings.Contains(out, "* glm") {
		t.Errorf("激活的 glm 应带 * 标记:\n%s", out)
	}
	// 非激活项无标记但应列出。
	if !strings.Contains(out, "deepseek") {
		t.Errorf("应列出 deepseek:\n%s", out)
	}
	// 应展示 GLM 的名称。
	if !strings.Contains(out, "GLM") {
		t.Errorf("应展示 GLM 名称:\n%s", out)
	}
	// 已激活时也应给出切换提示。
	if !strings.Contains(out, "switch to another provider") {
		t.Errorf("已激活时应给出切换提示:\n%s", out)
	}
}

func TestListProviders_NoneActive(t *testing.T) {
	t.Setenv(config.ActiveVar, "")
	cfg := &config.Config{Providers: map[string]config.Provider{"glm": {ID: "glm"}}}
	var b bytes.Buffer
	listProviders(&b, cfg)
	if !strings.Contains(b.String(), "no provider active in the current shell") {
		t.Errorf("未激活时应有提示:\n%s", b.String())
	}
}

func TestPrefilledFrom(t *testing.T) {
	oldEnv := map[string]string{
		"ANTHROPIC_BASE_URL": "https://old",
		"ANTHROPIC_MODEL":    "old-model",
	}
	old := config.Provider{ID: "glm", Name: "OldName"}
	// flag 全空 → 用旧值；name 空 → 用旧 name。
	got := prefilledFrom(oldEnv, old, addFlags{})
	if got.name != "OldName" || got.baseURL != "https://old" || got.model != "old-model" || got.preset != "custom" {
		t.Errorf("空 flag 应回填旧值: %+v", got)
	}
	// flag 显式提供 → 优先于旧值。
	got = prefilledFrom(oldEnv, old, addFlags{name: "New", baseURL: "https://new"})
	if got.name != "New" || got.baseURL != "https://new" || got.model != "old-model" {
		t.Errorf("显式 flag 应优先: %+v", got)
	}
}

func TestReadProviderInput_InteractiveFill(t *testing.T) {
	// flag 全空：从 stdin 依次读 base-url、model、api-key。
	in := strings.NewReader("https://x\nmodel-x\nsk-key\n")
	var out bytes.Buffer
	fl, err := readProviderInput(bufio.NewReader(in), &out, addFlags{}, "glm", "custom")
	if err != nil {
		t.Fatal(err)
	}
	if fl.name != "glm" {
		t.Errorf("name 默认应为 id，got %q", fl.name)
	}
	if fl.baseURL != "https://x" || fl.model != "model-x" {
		t.Errorf("交互读取失败: %+v", fl)
	}
	// api-key 通过 --field 或 stdin 处理；custom preset 下仍需输入。
}

func TestReadProviderInput_FlagsSkipPrompt(t *testing.T) {
	// base-url/model 由 flag 提供 → 不交互；api-key 仍从 stdin 读。
	in := strings.NewReader("sk-only\n")
	var out bytes.Buffer
	fl, err := readProviderInput(bufio.NewReader(in), &out, addFlags{baseURL: "https://flag", model: "flag-model"}, "glm", "custom")
	if err != nil {
		t.Fatal(err)
	}
	if fl.baseURL != "https://flag" || fl.model != "flag-model" {
		t.Errorf("flag 提供的字段不应被交互覆盖: %+v", fl)
	}
	// 提供了 flag 的字段不应打印对应 prompt。
	if strings.Contains(out.String(), "ANTHROPIC_BASE_URL") {
		t.Errorf("base-url 已由 flag 提供，不应再 prompt:\n%s", out.String())
	}
}

func TestWriteAndUpsertProvider(t *testing.T) {
	setTempCfg(t)
	a := &app.App{Config: config.Default(), Prefs: &prefs.Prefs{}, Secrets: secrets.NewFake()}
	fl := addFlags{name: "GLM", baseURL: "https://glm", model: "glm-4.6", apiKey: "sk-secret", preset: "custom"}
	// 用 ModeFull（全隔离）测试基础写入路径，避免触及 ~/.claude。
	if err := upsertProvider(a, "glm", fl, prefs.ModeFull); err != nil {
		t.Fatalf("upsertProvider: %v", err)
	}
	// providers.json 应含元信息。
	if _, ok := a.Config.Providers["glm"]; !ok {
		t.Error("config 应含 glm")
	}
	// profile settings.json 应含明文 env（含 token）。
	env, err := profile.ReadEnv("glm")
	if err != nil {
		t.Fatal(err)
	}
	if env["ANTHROPIC_BASE_URL"] != "https://glm" || env["ANTHROPIC_AUTH_TOKEN"] != "sk-secret" || env["ANTHROPIC_MODEL"] != "glm-4.6" {
		t.Errorf("profile env 写入不全: %v", env)
	}
}

// ---- runUse ----

func TestRunUse_Normal(t *testing.T) {
	setTempCfg(t)
	cfg := &config.Config{Providers: map[string]config.Provider{"glm": {ID: "glm", Name: "GLM"}}}
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Ensure("glm", map[string]string{"ANTHROPIC_BASE_URL": "https://glm"}); err != nil {
		t.Fatal(err)
	}

	resetFlags()
	useShellFlag = "zsh"
	cmd, out, eb := newCmd()
	if err := runUse(cmd, []string{"glm"}); err != nil {
		t.Fatalf("runUse: %v", err)
	}
	if !strings.Contains(out.String(), "export CLAUDE_CONFIG_DIR=") {
		t.Errorf("应导出 CLAUDE_CONFIG_DIR:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "export CC_SELECT_ACTIVE='glm'") {
		t.Errorf("应导出 CC_SELECT_ACTIVE=glm:\n%s", out.String())
	}
	// 提示走 stderr，不污染 eval。
	if !strings.Contains(eb.String(), "Switched to glm") {
		t.Errorf("stderr 应有切换提示:\n%s", eb.String())
	}
}

func TestRunUse_Official(t *testing.T) {
	setTempCfg(t)
	if err := config.Save(config.Default()); err != nil {
		t.Fatal(err)
	}
	resetFlags()
	useShellFlag = "zsh"
	cmd, out, _ := newCmd()
	if err := runUse(cmd, []string{config.OfficialProviderID}); err != nil {
		t.Fatalf("runUse 官方: %v", err)
	}
	// 官方：unset CLAUDE_CONFIG_DIR（回默认 ~/.claude）。
	if !strings.Contains(out.String(), "unset CLAUDE_CONFIG_DIR") {
		t.Errorf("官方应 unset CLAUDE_CONFIG_DIR:\n%s", out.String())
	}
}

func TestRunUse_MissingProvider(t *testing.T) {
	setTempCfg(t)
	config.Save(config.Default())
	resetFlags()
	cmd, _, _ := newCmd()
	if err := runUse(cmd, []string{"ghost"}); err == nil {
		t.Error("不存在的 provider 应报错")
	}
}

func TestRunUse_MissingProfile(t *testing.T) {
	setTempCfg(t)
	// 配置里有 glm，但不建 profile 目录 → use 应拒绝。
	cfg := &config.Config{Providers: map[string]config.Provider{"glm": {ID: "glm"}}}
	config.Save(cfg)
	resetFlags()
	useShellFlag = "zsh"
	cmd, _, _ := newCmd()
	err := runUse(cmd, []string{"glm"})
	if err == nil || !strings.Contains(err.Error(), "profile for provider \"glm\" is missing") {
		t.Errorf("profile 缺失应报错，got %v", err)
	}
}

func TestRunUse_OneOffFullMode(t *testing.T) {
	// --mode full 一次性：不落盘，但本次按 Mode A 构建（不报错、仍导出）。
	setTempCfg(t)
	cfg := &config.Config{Providers: map[string]config.Provider{"glm": {ID: "glm"}}}
	config.Save(cfg)
	if _, err := profile.Ensure("glm", map[string]string{"ANTHROPIC_BASE_URL": "https://glm"}); err != nil {
		t.Fatal(err)
	}
	resetFlags()
	useShellFlag = "zsh"
	useModeFlag = "full"
	cmd, out, _ := newCmd()
	if err := runUse(cmd, []string{"glm"}); err != nil {
		t.Fatalf("use --mode full: %v", err)
	}
	if !strings.Contains(out.String(), "export CLAUDE_CONFIG_DIR=") {
		t.Errorf("应导出 CLAUDE_CONFIG_DIR:\n%s", out.String())
	}
}

// ---- runInit ----

func TestRunInit_Zsh(t *testing.T) {
	resetFlags()
	initShellFlag = "zsh"
	cmd, out, _ := newCmd()
	if err := runInit(cmd); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	body := out.String()
	if !strings.Contains(body, "ccs()") {
		t.Errorf("init 应定义 ccs 函数:\n%s", body)
	}
	if !strings.Contains(body, "eval") {
		t.Errorf("init 应含 eval:\n%s", body)
	}
	// marker 化后，输出应含受管块标记（CLI 与 Web 共用 RenderInit）。
	if !strings.Contains(body, "cc-select shell integration") {
		t.Errorf("init 输出应含 marker 块:\n%s", body)
	}
}

// ---- 命令级（经 rootCmd 执行）----

func TestCurrentCommand_Active(t *testing.T) {
	setTempCfg(t)
	cfg := &config.Config{Providers: map[string]config.Provider{"glm": {ID: "glm", Name: "GLM"}}}
	config.Save(cfg)
	t.Setenv(config.ActiveVar, "glm")
	out, _, err := execRoot(t, "", "current")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "glm") || !strings.Contains(out, "GLM") {
		t.Errorf("current 应显示激活的 glm（GLM）:\n%s", out)
	}
}

func TestCurrentCommand_None(t *testing.T) {
	setTempCfg(t)
	t.Setenv(config.ActiveVar, "")
	out, _, err := execRoot(t, "", "current")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "none") {
		t.Errorf("未激活时 current 应输出 none:\n%s", out)
	}
}

func TestListCommand(t *testing.T) {
	setTempCfg(t)
	cfg := &config.Config{Providers: map[string]config.Provider{
		"glm": {ID: "glm", Name: "GLM"},
	}}
	config.Save(cfg)
	out, _, err := execRoot(t, "", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "glm") {
		t.Errorf("list 应列出 glm:\n%s", out)
	}
}

func TestAddCommand_EndToEnd(t *testing.T) {
	setTempCfg(t)
	config.Save(config.Default())
	// 使用 deepseek preset 并覆盖 model。
	out, _, err := execRoot(t, "", "add", "glm",
		"--preset", "deepseek", "--name", "GLM", "--model", "glm-4.6", "--api-key", "sk-ds")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !strings.Contains(out, "Added provider glm") {
		t.Errorf("add 应确认成功:\n%s", out)
	}
	// 重新加载验证落盘。
	cfg, _ := config.Load()
	if _, ok := cfg.Providers["glm"]; !ok {
		t.Error("glm 应已写入 config")
	}
	env, _ := profile.ReadEnv("glm")
	if env["ANTHROPIC_BASE_URL"] != "https://api.deepseek.com/anthropic" || env["ANTHROPIC_MODEL"] != "glm-4.6" {
		t.Errorf("profile env 应已写入: %v", env)
	}
	if env["ANTHROPIC_AUTH_TOKEN"] != "sk-ds" {
		t.Errorf("api key 应写入: %v", env)
	}
}

func TestAddCommand_DuplicateRejected(t *testing.T) {
	setTempCfg(t)
	cfg := &config.Config{Providers: map[string]config.Provider{"glm": {ID: "glm"}}}
	config.Save(cfg)
	_, _, err := execRoot(t, "", "add", "glm", "--preset", "deepseek", "--api-key", "sk-x")
	if err == nil {
		t.Error("重复 add 应报错")
	}
}

func TestAddCommand_RejectsBadID(t *testing.T) {
	setTempCfg(t)
	config.Save(config.Default())
	// 路径穿越 id 应被拒绝。
	_, _, err := execRoot(t, "", "add", "../../evil", "--preset", "deepseek", "--api-key", "sk-x")
	if err == nil {
		t.Error("路径穿越 id 应被拒绝")
	}
}

func TestEditCommand_UpdatesModel(t *testing.T) {
	setTempCfg(t)
	config.Save(config.Default())
	// 先 add（deepseek preset）。
	if _, _, err := execRoot(t, "", "add", "glm",
		"--preset", "deepseek", "--model", "old", "--api-key", "sk-ds"); err != nil {
		t.Fatal(err)
	}
	// 再 edit，仅改 model；api-key 留空=保持。
	if _, _, err := execRoot(t, "", "edit", "glm", "--model", "new-model"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	env, _ := profile.ReadEnv("glm")
	if env["ANTHROPIC_MODEL"] != "new-model" {
		t.Errorf("edit 应更新 model，got %v", env)
	}
	// base-url 未传 → 应保留旧值。
	if env["ANTHROPIC_BASE_URL"] != "https://api.deepseek.com/anthropic" {
		t.Errorf("未改字段应保留旧值，got %v", env)
	}
	// api-key 应保留。
	if env["ANTHROPIC_AUTH_TOKEN"] != "sk-ds" {
		t.Errorf("api key 应保留，got %v", env)
	}
}

func TestEditCommand_MissingProvider(t *testing.T) {
	setTempCfg(t)
	config.Save(config.Default())
	_, _, err := execRoot(t, "\n", "edit", "ghost", "--model", "x")
	if err == nil {
		t.Error("edit 不存在的 provider 应报错")
	}
}

func TestRemoveCommand(t *testing.T) {
	setTempCfg(t)
	cfg := &config.Config{Providers: map[string]config.Provider{
		config.OfficialProviderID: config.Default().Providers[config.OfficialProviderID],
		"glm":                     {ID: "glm"},
	}}
	config.Save(cfg)
	profile.Ensure("glm", map[string]string{"ANTHROPIC_BASE_URL": "https://glm"})

	out, _, err := execRoot(t, "", "remove", "glm")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !strings.Contains(out, "Removed provider glm") {
		t.Errorf("remove 应确认成功:\n%s", out)
	}
	cfg2, _ := config.Load()
	if _, ok := cfg2.Providers["glm"]; ok {
		t.Error("glm 应已从 config 删除")
	}
	if ok, _ := profile.Exists("glm"); ok {
		t.Error("glm 的 profile 应已删除")
	}
}

func TestRemoveCommand_RejectsOfficial(t *testing.T) {
	setTempCfg(t)
	config.Save(config.Default())
	_, _, err := execRoot(t, "", "remove", config.OfficialProviderID)
	if err == nil {
		t.Error("删除官方 provider 应报错")
	}
}

func TestRemoveCommand_MissingProvider(t *testing.T) {
	setTempCfg(t)
	config.Save(config.Default())
	_, _, err := execRoot(t, "", "remove", "ghost")
	if err == nil {
		t.Error("remove 不存在的 provider 应报错")
	}
}

// ---- Execute（顶层退出码）----

func TestExecute_ReturnCodes(t *testing.T) {
	setTempCfg(t)
	config.Save(config.Default())

	// 正常子命令 → 0。
	resetFlags()
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"list"})
	if code := Execute(); code != 0 {
		t.Errorf("list 应返回退出码 0，got %d", code)
	}

	// 未知子命令 → 1。
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})
	rootCmd.SetArgs([]string{"no-such-command"})
	if code := Execute(); code != 1 {
		t.Errorf("未知命令应返回退出码 1，got %d", code)
	}
}

// ---- mode 命令 / per-provider 模式 ----

func TestModeCommand_DefaultIsSettingsOnly(t *testing.T) {
	setTempCfg(t)
	out, _, err := execRoot(t, "", "mode")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "settings-only") {
		t.Errorf("默认全局模式应为 settings-only: %s", out)
	}
}

func TestModeCommand_SetAndPersist(t *testing.T) {
	setTempCfg(t)
	if _, _, err := execRoot(t, "", "mode", "full"); err != nil {
		t.Fatalf("mode full: %v", err)
	}
	pr, _ := prefs.Load()
	if pr.IsolationMode != prefs.ModeFull {
		t.Errorf("全局模式应落盘 full, got %q", pr.IsolationMode)
	}
	out, _, _ := execRoot(t, "", "mode")
	if !strings.Contains(out, "full") {
		t.Errorf("再读应显示 full: %s", out)
	}
}

func TestModeCommand_RejectsInvalid(t *testing.T) {
	setTempCfg(t)
	if _, _, err := execRoot(t, "", "mode", "bogus"); err == nil {
		t.Error("无效模式应报错")
	}
}

func TestAddCommand_PerProviderMode(t *testing.T) {
	setTempCfg(t)
	config.Save(config.Default())
	if _, _, err := execRoot(t, "", "add", "glm",
		"--preset", "deepseek", "--api-key", "sk-ds", "--mode", "full"); err != nil {
		t.Fatalf("add --mode full: %v", err)
	}
	cfg, _ := config.Load()
	if got := cfg.Providers["glm"].IsolationMode; got != prefs.ModeFull {
		t.Errorf("per-provider 模式应落盘 full, got %q", got)
	}
}

func TestEditCommand_DefaultClearsOverride(t *testing.T) {
	setTempCfg(t)
	config.Save(config.Default())
	// 先设 per-provider full。
	if _, _, err := execRoot(t, "", "add", "glm", "--preset", "deepseek", "--api-key", "sk-ds", "--mode", "full"); err != nil {
		t.Fatal(err)
	}
	// edit --mode default 清除覆盖（继承全局）。
	if _, _, err := execRoot(t, "", "edit", "glm", "--mode", "default"); err != nil {
		t.Fatalf("edit --mode default: %v", err)
	}
	cfg, _ := config.Load()
	if got := cfg.Providers["glm"].IsolationMode; got != "" {
		t.Errorf("default 应清除覆盖（空=继承全局）, got %q", got)
	}
}

// TestListProviders_ChineseOutput 验证设置中文 locale 后 CLI 输出中文提示。
func TestAddCommand_InteractivePresetSelect(t *testing.T) {
	setTempCfg(t)
	config.Save(config.Default())
	// 输入 preset id；baseURL/model 留空使用 preset 默认值；最后输入 api key。
	out, _, err := execRoot(t, "deepseek\n\n\nsk-ds\n", "add", "ds")
	if err != nil {
		t.Fatalf("add interactive preset: %v", err)
	}
	if !strings.Contains(out, "Added provider ds") {
		t.Errorf("add 应确认成功:\n%s", out)
	}
	env, _ := profile.ReadEnv("ds")
	if env["ANTHROPIC_BASE_URL"] != "https://api.deepseek.com/anthropic" {
		t.Errorf("deepseek preset 默认值未应用: %v", env)
	}
}

func TestAddCommand_CustomFieldOverride(t *testing.T) {
	setTempCfg(t)
	config.Save(config.Default())
	_, _, err := execRoot(t, "", "add", "ds",
		"--preset", "deepseek", "--api-key", "sk-ds",
		"--field", "ANTHROPIC_DEFAULT_SONNET_MODEL=sonnet-5",
		"--field", "CLAUDE_CODE_SUBAGENT_MODEL=sub-1")
	if err != nil {
		t.Fatalf("add custom field: %v", err)
	}
	env, _ := profile.ReadEnv("ds")
	if env["ANTHROPIC_DEFAULT_SONNET_MODEL"] != "sonnet-5" {
		t.Errorf("自定义模型映射字段未写入: %v", env)
	}
	if env["CLAUDE_CODE_SUBAGENT_MODEL"] != "sub-1" {
		t.Errorf("subagent 模型字段未写入: %v", env)
	}
}

func TestAddCommand_OAuthNoKey(t *testing.T) {
	setTempCfg(t)
	config.Save(config.Default())
	_, _, err := execRoot(t, "", "add", "copilot", "--preset", "github-copilot")
	if err != nil {
		t.Fatalf("add oauth preset: %v", err)
	}
	env, _ := profile.ReadEnv("copilot")
	if env["ANTHROPIC_BASE_URL"] != "https://api.githubcopilot.com" {
		t.Errorf("OAuth preset base url 未写入: %v", env)
	}
}

func TestEditCommand_AddRemoveField(t *testing.T) {
	setTempCfg(t)
	config.Save(config.Default())
	if _, _, err := execRoot(t, "", "add", "ds",
		"--preset", "deepseek", "--api-key", "sk-ds"); err != nil {
		t.Fatal(err)
	}
	// 添加一个 preset 模板中不存在的自定义字段。
	if _, _, err := execRoot(t, "", "edit", "ds",
		"--add-field", "CUSTOM_VAR=hello"); err != nil {
		t.Fatalf("edit add-field: %v", err)
	}
	env, _ := profile.ReadEnv("ds")
	if env["CUSTOM_VAR"] != "hello" {
		t.Errorf("add-field 未生效: %v", env)
	}
	if _, _, err := execRoot(t, "", "edit", "ds",
		"--remove-field", "CUSTOM_VAR"); err != nil {
		t.Fatalf("edit remove-field: %v", err)
	}
	env, _ = profile.ReadEnv("ds")
	if _, ok := env["CUSTOM_VAR"]; ok {
		t.Errorf("remove-field 未删除字段: %v", env)
	}
}

func TestListProviders_ChineseOutput(t *testing.T) {
	setTempCfg(t)
	i18n.SetLocale(i18n.ZH)
	t.Cleanup(func() { i18n.SetLocale(i18n.EN) })
	t.Setenv(config.ActiveVar, "")
	cfg := &config.Config{Providers: map[string]config.Provider{
		"glm": {ID: "glm", Name: "GLM"},
	}}
	var b bytes.Buffer
	listProviders(&b, cfg)
	out := b.String()
	if !strings.Contains(out, "未激活任何 provider") {
		t.Errorf("中文环境下应显示未激活提示:\n%s", out)
	}
}
