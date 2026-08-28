package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/config"

	"github.com/cc-select/cc-select/internal/profile"
	"github.com/cc-select/cc-select/internal/routes"
	"github.com/cc-select/cc-select/internal/secrets"
)

// stubEnsure 替换 ensureRouterFn，记录调用并返回预设结果。
func stubEnsure(t *testing.T, addr string, err error) *int {
	t.Helper()
	calls := 0
	orig := ensureRouterFn
	ensureRouterFn = func() (string, error) {
		calls++
		return addr, err
	}
	t.Cleanup(func() { ensureRouterFn = orig })
	return &calls
}

func TestUse_ProxyMode_EmissionAndProfile(t *testing.T) {
	setTempCfg(t)
	writeProviders(t)
	calls := stubEnsure(t, "127.0.0.1:48270", nil)

	out, _, err := execRoot(t, "", "use", "glm", "--mode", "proxy", "--shell", "zsh")
	if err != nil {
		t.Fatalf("use --mode proxy: %v（out=%s）", err, out)
	}
	if *calls != 1 {
		t.Errorf("应恰好 ensure 一次，got %d", *calls)
	}
	// 发射契约（contracts/cli.md §3）：TID 守卫 → 引用式 AUTH_TOKEN → CONFIG_DIR → ACTIVE → 路由同步。
	for _, want := range []string{
		"if [ -z \"${CC_SELECT_TID:-}\" ]; then",
		"export CC_SELECT_TID='ccs-",
		"export ANTHROPIC_AUTH_TOKEN=\"$CC_SELECT_TID\"",
		"export CLAUDE_CONFIG_DIR=",
		"export CC_SELECT_ACTIVE='glm'",
		"route switch glm >/dev/null 2>&1 || true",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("发射缺少 %q:\n%s", want, out)
		}
	}
	// profile：env 仅含恒定 BASE_URL（指向 ensure 返回的 addr）。
	env, err := profile.ReadEnv("glm")
	if err != nil {
		t.Fatal(err)
	}
	if len(env) != 1 || env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:48270" {
		t.Errorf("profile env 应仅含代理 BASE_URL: %+v", env)
	}
	// 路由表：二进制不直接写——由 eval 上下文里的同步命令按 shell 真实 TID 写。
	if entries, _ := routes.List(); len(entries) != 0 {
		t.Errorf("use 二进制不应写路由表（eval 同步命令负责）: %+v", entries)
	}
}

func TestUse_ProxyMode_EnsureFailsAborts(t *testing.T) {
	setTempCfg(t)
	writeProviders(t)
	stubEnsure(t, "", errRouterStub)

	out, _, err := execRoot(t, "", "use", "glm", "--mode", "proxy", "--shell", "zsh")
	if err == nil {
		t.Fatalf("ensure 失败应中止（FR-011）: out=%s", out)
	}
	if strings.Contains(out, "export") {
		t.Errorf("失败时不得发射任何语句: %s", out)
	}
	if ok, _ := profile.Exists("glm"); ok {
		t.Error("失败时不应创建 profile")
	}
}

func TestUse_ProxyMode_OfficialFallsBack(t *testing.T) {
	setTempCfg(t)
	writeProviders(t)
	calls := stubEnsure(t, "127.0.0.1:48270", nil)

	out, _, err := execRoot(t, "", "use", config.OfficialProviderID, "--mode", "proxy", "--shell", "zsh")
	if err != nil {
		t.Fatalf("官方回退应成功: %v", err)
	}
	if *calls != 0 {
		t.Error("官方目标不应拉起 daemon（D2）")
	}
	for _, want := range []string{"unset CLAUDE_CONFIG_DIR", "unset ANTHROPIC_AUTH_TOKEN"} {
		if !strings.Contains(out, want) {
			t.Errorf("官方回退缺少 %q: %s", want, out)
		}
	}
	if strings.Contains(out, "cc-select route switch") {
		t.Errorf("官方回退不应有路由同步语句: %s", out)
	}
}

// errRouterStub 是测试用的 ensure 失败哨兵。
var errRouterStub = &routerStubError{}

type routerStubError struct{}

func (*routerStubError) Error() string { return "router stub failure" }

func TestUse_ProxyThenSettingsOnly_RestoresProviderEnv(t *testing.T) {
	setTempCfg(t)
	// legacy 形态：真值只在 profile（providers.json 无 env）。
	writeProviders(t)
	os.WriteFile(os.Getenv("CC_SELECT_CONFIG"),
		[]byte(`{"providers":{"glm":{"id":"glm","name":"GLM"}}}`), 0o600)
	if _, err := profile.Ensure("glm", map[string]string{"ANTHROPIC_BASE_URL": "https://glm.api", "ANTHROPIC_AUTH_TOKEN": "sk-real"}); err != nil {
		t.Fatal(err)
	}
	stubEnsure(t, "127.0.0.1:48270", nil)
	// 迁移与占位解析都走 FakeStore（严禁测试触碰真实 keychain）。
	store := secrets.NewFake()
	origMig := migrateSecretsFn
	migrateSecretsFn = func(cfg *config.Config) (int, []string) { return secrets.MigrateAll(store, cfg) }
	origGet := profile.SecretGetter
	profile.SecretGetter = func(svc string) (string, error) { return store.Get(svc) }
	t.Cleanup(func() {
		migrateSecretsFn = origMig
		profile.SecretGetter = origGet
	})

	// 进 proxy：profile env 被替换为代理 BASE_URL（真值上抬进 providers.json）。
	if _, _, err := execRoot(t, "", "use", "glm", "--mode", "proxy", "--shell", "zsh"); err != nil {
		t.Fatalf("use proxy: %v", err)
	}
	if env, _ := profile.ReadEnv("glm"); env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:48270" {
		t.Fatalf("proxy profile 应指向代理: %+v", env)
	}

	// 切回 Mode B：profile env 应恢复为 provider 真值（评审 #1 往返污染修复）。
	if _, _, err := execRoot(t, "", "use", "glm", "--mode", "settings-only", "--shell", "zsh"); err != nil {
		t.Fatalf("use settings-only: %v", err)
	}
	env, err := profile.ReadEnv("glm")
	if err != nil {
		t.Fatal(err)
	}
	if env["ANTHROPIC_BASE_URL"] != "https://glm.api" || env["ANTHROPIC_AUTH_TOKEN"] != "sk-real" {
		t.Errorf("切回 Mode B 应恢复 provider env，而非残留代理 BASE_URL: %+v", env)
	}
}
