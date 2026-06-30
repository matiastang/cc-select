package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/prefs"
)

// setTempClaudeHome 建一个临时目录作为 ~/.claude（被共享的源），返回其路径。
func setTempClaudeHome(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	t.Setenv("CC_SELECT_CLAUDE_HOME", d)
	return d
}

func readProfileSettings(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("读 profile settings: %v", err)
	}
	return string(b)
}

func TestMergeSettings_PreservesUnknownAndReplacesEnv(t *testing.T) {
	global := []byte(`{"permissions":{"allow":["foo"]},"env":{"ANTHROPIC_BASE_URL":"OLD","KEEP":"v"},"model":"x"}`)
	env := map[string]string{"ANTHROPIC_BASE_URL": "NEW"}
	out, err := mergeSettings(global, env)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// 未知字段保留。
	if !strings.Contains(s, "permissions") || !strings.Contains(s, `"model":`) {
		t.Errorf("应保留未知字段: %s", s)
	}
	// env 整体替换：NEW 在，OLD 与 KEEP 不在（非深合并）。
	if !strings.Contains(s, "NEW") {
		t.Errorf("应含新 env: %s", s)
	}
	if strings.Contains(s, "OLD") || strings.Contains(s, `"KEEP"`) {
		t.Errorf("env 应整体替换、不留全局 env: %s", s)
	}
}

func TestMergeSettings_EmptyGlobal(t *testing.T) {
	out, err := mergeSettings(nil, map[string]string{"K": "v"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"K":`) {
		t.Errorf("空全局应得 {env:...}: %s", out)
	}
}

func TestMergeSettings_InvalidGlobal(t *testing.T) {
	if _, err := mergeSettings([]byte(`{not json`), map[string]string{"K": "v"}); err == nil {
		t.Error("非法全局应返回错误")
	}
}

func TestSync_Full_EquivEnsure(t *testing.T) {
	setTempRoot(t)
	setTempClaudeHome(t)
	env := map[string]string{"ANTHROPIC_MODEL": "glm"}
	dir, warns, err := Sync("glm", env, prefs.ModeFull)
	if err != nil {
		t.Fatalf("Sync Full: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("Full 不应有 warning: %v", warns)
	}
	body := readProfileSettings(t, dir)
	if !strings.Contains(body, "glm") {
		t.Errorf("应写 env: %s", body)
	}
	// Full 模式目录除 settings.json 外无其他条目（隔离）。
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if e.Name() != "settings.json" {
			t.Errorf("Full 模式不应有额外条目 %s", e.Name())
		}
	}
}

func TestSync_SettingsOnly_MergesAndShares(t *testing.T) {
	root := setTempRoot(t)
	_ = root
	home := setTempClaudeHome(t)
	// 准备 claude home：全局 settings（含 permissions）+ projects 目录。
	os.MkdirAll(filepath.Join(home, "projects"), 0o700)
	os.WriteFile(filepath.Join(home, "settings.json"),
		[]byte(`{"permissions":{"allow":["foo"]}}`), 0o600)

	env := map[string]string{"ANTHROPIC_MODEL": "glm"}
	dir, _, err := Sync("glm", env, prefs.ModeSettingsOnly)
	if err != nil {
		t.Fatalf("Sync B: %v", err)
	}

	// settings.json 合并：env 来自 provider，permissions 来自全局。
	body := readProfileSettings(t, dir)
	if !strings.Contains(body, "glm") || !strings.Contains(body, "permissions") {
		t.Errorf("合并 settings 应含 env 与全局 permissions: %s", body)
	}

	// 共享穿透：经 profile/projects 写入，应落到 claudeHome/projects（共享生效）。
	if err := os.WriteFile(filepath.Join(dir, "projects", "chat.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatalf("经链接写入 projects: %v（可能无链接权限）", err)
	}
	got, err := os.ReadFile(filepath.Join(home, "projects", "chat.txt"))
	if err != nil {
		t.Fatalf("共享目录应能在 claudeHome 读到: %v", err)
	}
	if string(got) != "hi" {
		t.Errorf("共享内容不符: %q", got)
	}
}

func TestSync_SettingsOnly_NoClaudeHomeDegrades(t *testing.T) {
	setTempRoot(t)
	home := setTempClaudeHome(t)
	// home 存在但为空：无 settings、无条目。
	_ = home
	dir, warns, err := Sync("glm", map[string]string{"ANTHROPIC_MODEL": "glm"}, prefs.ModeSettingsOnly)
	if err != nil {
		t.Fatalf("空 ~/.claude 不应报错: %v", err)
	}
	body := readProfileSettings(t, dir)
	if !strings.Contains(body, "glm") {
		t.Errorf("降级应仍写 env: %s", body)
	}
	// 空全局 → 无 permissions 可合并，不应产生致命 warning（可能仅有悬挂链接类提示）。
	_ = warns
}

func TestSync_FullPrunesLeftoverLinks(t *testing.T) {
	setTempRoot(t)
	home := setTempClaudeHome(t)
	os.MkdirAll(filepath.Join(home, "projects"), 0o700)

	env := map[string]string{"ANTHROPIC_MODEL": "glm"}
	// 先 B：建立 projects 链接。
	dir, _, err := Sync("glm", env, prefs.ModeSettingsOnly)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "projects")); err != nil {
		t.Fatalf("B 模式应建立 projects 链接: %v", err)
	}
	// 再切 A：应清理链接，只剩 settings.json。
	if _, _, err := Sync("glm", env, prefs.ModeFull); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "projects")); !os.IsNotExist(err) {
		t.Errorf("Full 模式应清理掉 projects 链接, err=%v", err)
	}
}

func TestSync_NilEnvReadsExisting(t *testing.T) {
	// use 路径：env=nil 沿用现有 profile env。
	setTempRoot(t)
	setTempClaudeHome(t)
	if _, _, err := Sync("glm", map[string]string{"ANTHROPIC_MODEL": "keep"}, prefs.ModeFull); err != nil {
		t.Fatal(err)
	}
	// nil env + 缺失 profile 应报错。
	if _, _, err := Sync("absent", nil, prefs.ModeFull); err == nil {
		t.Error("nil env 且 profile 缺失应报错")
	}
	// nil env + 存在的 profile：沿用 env，仍含 keep。
	dir, _, err := Sync("glm", nil, prefs.ModeFull)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readProfileSettings(t, dir), "keep") {
		t.Error("nil env 应沿用现有 env")
	}
}

func TestSync_SettingsOnly_SharesClaudeJSONSibling(t *testing.T) {
	// .claude.json 在 home 根（~/.claude.json，是 ~/.claude 的 sibling），不在 ~/.claude/ 内。
	// 用嵌套 tempdir 模拟：root/.claude (=claudeHome) + root/.claude.json (sibling)。
	root := t.TempDir()
	claudeHome := filepath.Join(root, ".claude")
	os.MkdirAll(filepath.Join(claudeHome, "projects"), 0o700)
	os.WriteFile(filepath.Join(root, ".claude.json"), []byte(`{"oauth":"acct-X"}`), 0o600)
	setTempRoot(t)
	t.Setenv("CC_SELECT_CLAUDE_HOME", claudeHome)

	dir, _, err := Sync("glm", map[string]string{"ANTHROPIC_MODEL": "glm"}, prefs.ModeSettingsOnly)
	if err != nil {
		t.Fatalf("Sync B: %v", err)
	}
	// profile/.claude.json 应软链到 home 根 sibling，内容可达（共享生效）。
	got, err := os.ReadFile(filepath.Join(dir, ".claude.json"))
	if err != nil {
		t.Fatalf("读 profile/.claude.json: %v（应已软链共享）", err)
	}
	if !strings.Contains(string(got), "acct-X") {
		t.Errorf(".claude.json 应共享 home 根 sibling: %s", got)
	}
}

func TestSync_OfficialNoop(t *testing.T) {
	setTempRoot(t)
	dir, _, err := Sync(config.OfficialProviderID, map[string]string{"X": "1"}, prefs.ModeSettingsOnly)
	if err != nil {
		t.Fatalf("官方应 no-op: %v", err)
	}
	if dir != "" {
		t.Errorf("官方应返回空目录 got %q", dir)
	}
}
