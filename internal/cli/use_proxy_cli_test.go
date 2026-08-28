package cli

import (
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/config"

	"github.com/cc-select/cc-select/internal/profile"
	"github.com/cc-select/cc-select/internal/routes"
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
		"cc-select route switch glm >/dev/null 2>&1 || true",
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
