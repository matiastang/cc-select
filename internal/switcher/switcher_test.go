package switcher

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/cc-select/cc-select/internal/shell"
)

// setTempRoot 隔离 profiles 落点（CC_SELECT_CONFIG 指向 tempdir）。
func setTempRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CC_SELECT_CONFIG", filepath.Join(dir, "providers.json"))
	return dir
}

func TestPlan_NormalProviderSetsConfigDir(t *testing.T) {
	setTempRoot(t)
	p := config.Provider{ID: "minimax", Name: "MiniMax"}

	changes := Plan(p)

	wantDir, _ := profile.Dir("minimax")
	setByName := map[string]string{}
	for _, c := range changes {
		if c.Op == shell.OpSet {
			setByName[c.Name] = c.Value
		}
	}
	if setByName[profile.ConfigVar] != wantDir {
		t.Errorf("应 set CLAUDE_CONFIG_DIR=%s，got %q", wantDir, setByName[profile.ConfigVar])
	}
	if setByName[config.ActiveVar] != "minimax" {
		t.Errorf("应 set CC_SELECT_ACTIVE=minimax，got %q", setByName[config.ActiveVar])
	}
}

func TestPlan_NormalProviderNoAnthropicVars(t *testing.T) {
	// 回归保护：新机制不应再产出任何 ANTHROPIC_* 变更。
	setTempRoot(t)
	changes := Plan(config.Provider{ID: "glm", Env: map[string]string{"ANTHROPIC_BASE_URL": "x"}})

	for _, c := range changes {
		if len(c.Name) >= len("ANTHROPIC_") && c.Name[:len("ANTHROPIC_")] == "ANTHROPIC_" {
			t.Errorf("新机制不应产出 ANTHROPIC_* 变更，got %+v", c)
		}
	}
}

func TestPlan_OfficialProviderUnsetsConfigDir(t *testing.T) {
	setTempRoot(t)
	changes := Plan(config.Provider{ID: config.OfficialProviderID})

	// 应含 unset CLAUDE_CONFIG_DIR。
	var hasUnset bool
	var hasSetConfigDir bool
	for _, c := range changes {
		if c.Name == profile.ConfigVar {
			if c.Op == shell.OpUnset {
				hasUnset = true
			}
			if c.Op == shell.OpSet {
				hasSetConfigDir = true
			}
		}
	}
	if !hasUnset {
		t.Error("官方 provider 应 unset CLAUDE_CONFIG_DIR")
	}
	if hasSetConfigDir {
		t.Error("官方 provider 不应 set CLAUDE_CONFIG_DIR")
	}

	// 仍应 set active。
	var hasActive bool
	for _, c := range changes {
		if c.Op == shell.OpSet && c.Name == config.ActiveVar {
			hasActive = true
		}
	}
	if !hasActive {
		t.Error("官方 provider 仍应 set CC_SELECT_ACTIVE")
	}
}

func TestPlanProxy_EmissionOrder(t *testing.T) {
	setTempRoot(t)
	tid := "ccs-" + strings.Repeat("ab", 16)
	changes := PlanProxy(config.Provider{ID: "minimax", Name: "MiniMax"}, tid)

	if len(changes) != 4 {
		t.Fatalf("应产出 4 条变更，got %d: %+v", len(changes), changes)
	}
	// 顺序（contracts/cli.md §3）：TID 守卫 → AUTH_TOKEN 引用 → CLAUDE_CONFIG_DIR → CC_SELECT_ACTIVE。
	if changes[0].Op != shell.OpSetIfUnset || changes[0].Name != config.TerminalIDVar || changes[0].Value != tid {
		t.Errorf("第 1 条应为 TID 守卫导出，got %+v", changes[0])
	}
	if changes[1].Op != shell.OpSetRef || changes[1].Name != config.AuthTokenVar || changes[1].Value != config.TerminalIDVar {
		t.Errorf("第 2 条应为 AUTH_TOKEN 引用 TID，got %+v", changes[1])
	}
	wantDir, _ := profile.Dir("minimax")
	if changes[2].Op != shell.OpSet || changes[2].Name != profile.ConfigVar || changes[2].Value != wantDir {
		t.Errorf("第 3 条应为 CLAUDE_CONFIG_DIR 指向 profile，got %+v", changes[2])
	}
	if changes[3].Op != shell.OpSet || changes[3].Name != config.ActiveVar || changes[3].Value != "minimax" {
		t.Errorf("第 4 条应为 CC_SELECT_ACTIVE，got %+v", changes[3])
	}
}

func TestPlanProxy_ZshRender(t *testing.T) {
	// 锁定 contracts/cli.md §3 的 zsh 发射契约原文。
	setTempRoot(t)
	tid := "ccs-" + strings.Repeat("ab", 16)
	out := shell.ZshEmitter{}.Emit(PlanProxy(config.Provider{ID: "minimax"}, tid))
	wantDir, _ := profile.Dir("minimax")
	want := fmt.Sprintf(
		"if [ -z \"${%s:-}\" ]; then\n  export %s='%s'\nfi\nexport %s=\"$%s\"\nexport %s='%s'\nexport %s='minimax'\n",
		config.TerminalIDVar, config.TerminalIDVar, tid,
		config.AuthTokenVar, config.TerminalIDVar,
		profile.ConfigVar, wantDir,
		config.ActiveVar)
	if out != want {
		t.Errorf("zsh 发射契约:\nwant %q\ngot  %q", want, out)
	}
}

func TestPlanProxy_OfficialFallbackCleansPseudoToken(t *testing.T) {
	setTempRoot(t)
	tid := "ccs-" + strings.Repeat("ab", 16)
	changes := PlanProxy(config.Provider{ID: config.OfficialProviderID}, tid)

	var hasUnsetConfigDir, hasUnsetAuth, hasGuard, hasActive bool
	for _, c := range changes {
		switch {
		case c.Name == profile.ConfigVar && c.Op == shell.OpUnset:
			hasUnsetConfigDir = true
		case c.Name == config.AuthTokenVar && c.Op == shell.OpUnset:
			hasUnsetAuth = true
		case c.Op == shell.OpSetIfUnset:
			hasGuard = true
		case c.Op == shell.OpSet && c.Name == config.ActiveVar:
			hasActive = true
		}
	}
	if !hasUnsetConfigDir {
		t.Error("官方应 unset CLAUDE_CONFIG_DIR")
	}
	if !hasUnsetAuth {
		t.Error("官方应 unset ANTHROPIC_AUTH_TOKEN（伪 token 不得发给官方端点）")
	}
	if hasGuard {
		t.Error("官方回退不应产出 TID 守卫")
	}
	if !hasActive {
		t.Error("官方仍应 set CC_SELECT_ACTIVE")
	}
}
