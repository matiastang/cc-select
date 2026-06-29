package switcher

import (
	"path/filepath"
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
