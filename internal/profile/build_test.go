package profile

import (
	"os"
	"path/filepath"
	"runtime"
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

	dir, warns, err := Sync("glm", map[string]string{"ANTHROPIC_MODEL": "glm"}, prefs.ModeSettingsOnly)
	if err != nil {
		t.Fatalf("Sync B: %v", err)
	}
	// profile/.claude.json 应软链到 home 根 sibling，内容可达（共享生效）。
	// Windows 文件符号链接需开发者模式或管理员权限；无权限时生产代码会降级并告警，
	// 测试验证该降级行为而非强制要求链接成功。
	got, err := os.ReadFile(filepath.Join(dir, ".claude.json"))
	if err != nil {
		if runtime.GOOS != "windows" {
			t.Fatalf("读 profile/.claude.json: %v（应已软链共享）", err)
		}
		found := false
		for _, w := range warns {
			if strings.Contains(w, ".claude.json") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Windows 无符号链接权限时应告警 .claude.json，warnings=%v", warns)
		}
		src, serr := os.ReadFile(filepath.Join(root, ".claude.json"))
		if serr != nil {
			t.Fatalf("读源 .claude.json: %v", serr)
		}
		if !strings.Contains(string(src), "acct-X") {
			t.Errorf("源 .claude.json 内容丢失: %s", src)
		}
		return
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

func TestSyncProxy_BaseURLOnlyAndShares(t *testing.T) {
	setTempRoot(t)
	home := setTempClaudeHome(t)
	os.MkdirAll(filepath.Join(home, "projects"), 0o700)
	// 全局 settings 带遗留 env（含 token）——Mode P 必须整体替换屏蔽，不得泄漏进 profile。
	os.WriteFile(filepath.Join(home, "settings.json"),
		[]byte(`{"permissions":{"allow":["foo"]},"env":{"ANTHROPIC_BASE_URL":"https://old.example","ANTHROPIC_AUTH_TOKEN":"sk-leak"}}`), 0o600)

	dir, _, err := SyncProxy("glm", "http://127.0.0.1:48270")
	if err != nil {
		t.Fatalf("SyncProxy: %v", err)
	}
	body := readProfileSettings(t, dir)
	// env 仅含恒定的代理 BASE_URL。
	if !strings.Contains(body, `"ANTHROPIC_BASE_URL": "http://127.0.0.1:48270"`) {
		t.Errorf("env 应仅含代理 BASE_URL: %s", body)
	}
	if strings.Contains(body, "sk-leak") || strings.Contains(body, "old.example") {
		t.Errorf("全局遗留 env（含 token）不得残留在 profile: %s", body)
	}
	if strings.Contains(body, "ANTHROPIC_AUTH_TOKEN") {
		t.Errorf("Mode P profile 不得含 AUTH_TOKEN（伪 token 走 shell 注入）: %s", body)
	}
	// 全局非 env 字段保留（Mode B 合并语义复用）。
	if !strings.Contains(body, "permissions") {
		t.Errorf("permissions 应保留: %s", body)
	}
	// 共享链接仍生效（上下文延续的物质基础）。
	if _, err := os.Lstat(filepath.Join(dir, "projects")); err != nil {
		t.Errorf("projects 共享链接应建立: %v", err)
	}
}

func TestSyncProxy_IdempotentAddrHeal(t *testing.T) {
	setTempRoot(t)
	setTempClaudeHome(t)
	dir1, _, err := SyncProxy("glm", "http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("首次 SyncProxy: %v", err)
	}
	dir2, _, err := SyncProxy("glm", "http://127.0.0.2:2")
	if err != nil {
		t.Fatalf("二次 SyncProxy: %v", err)
	}
	if dir1 != dir2 {
		t.Errorf("幂等：目录应一致 %q vs %q", dir1, dir2)
	}
	body := readProfileSettings(t, dir2)
	if !strings.Contains(body, "127.0.0.2:2") || strings.Contains(body, "127.0.0.1:1") {
		t.Errorf("重复 SyncProxy 应自愈为最新 addr: %s", body)
	}
}

func TestSyncProxy_OfficialNoop(t *testing.T) {
	setTempRoot(t)
	dir, _, err := SyncProxy(config.OfficialProviderID, "http://127.0.0.1:48270")
	if err != nil {
		t.Fatalf("官方应 no-op: %v", err)
	}
	if dir != "" {
		t.Errorf("官方应返回空目录 got %q", dir)
	}
}

func TestSync_ProxyModeRejected(t *testing.T) {
	// ModeProxy 不能走 Sync（env 真值是路由地址而非 provider env）——防误用。
	setTempRoot(t)
	setTempClaudeHome(t)
	if _, _, err := Sync("glm", map[string]string{"X": "1"}, prefs.ModeProxy); err == nil {
		t.Error("Sync + ModeProxy 应报错引导使用 SyncProxy")
	}
}

func TestSyncProxy_UpliftsLegacyEnvToProviders(t *testing.T) {
	setTempRoot(t)
	setTempClaudeHome(t)
	realEnv := map[string]string{"ANTHROPIC_BASE_URL": "https://glm.api", "ANTHROPIC_AUTH_TOKEN": "sk-real"}
	if _, err := Ensure("glm", realEnv); err != nil { // legacy：env 只在 profile（Mode A 形态）
		t.Fatal(err)
	}
	writeProvidersJSON(t, `{"providers":{"glm":{"id":"glm"}}}`) // providers.json 无 env

	if _, _, err := SyncProxy("glm", "http://127.0.0.1:48270"); err != nil {
		t.Fatalf("SyncProxy: %v", err)
	}
	// 真值已上抬：providers.json 拿到完整 env（此后切回 Mode B 可恢复）。
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Providers["glm"].Env
	if got["ANTHROPIC_BASE_URL"] != "https://glm.api" || got["ANTHROPIC_AUTH_TOKEN"] != "sk-real" {
		t.Errorf("SyncProxy 应先上抬 legacy env 到 providers.json（防往返丢失）: %+v", got)
	}
	// 二次 SyncProxy 不重复上抬（幂等）。
	if _, _, err := SyncProxy("glm", "http://127.0.0.1:48270"); err != nil {
		t.Fatal(err)
	}
	cfg2, _ := config.Load()
	if cfg2.Providers["glm"].Env["ANTHROPIC_BASE_URL"] != "https://glm.api" {
		t.Errorf("幂等上抬: %+v", cfg2.Providers["glm"].Env)
	}
}

// writeProvidersJSON 写一份 providers.json 到 CC_SELECT_CONFIG 指向位置。
func writeProvidersJSON(t *testing.T, json string) {
	t.Helper()
	p := os.Getenv("CC_SELECT_CONFIG")
	if p == "" {
		t.Fatal("先 setTempRoot")
	}
	if err := os.WriteFile(p, []byte(json), 0o600); err != nil {
		t.Fatal(err)
	}
}
