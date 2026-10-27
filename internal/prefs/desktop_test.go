package prefs

import (
	"os"
	"path/filepath"
	"testing"
)

// T006（specs/003-desktop-client/tasks.md）：Prefs 扩展 Desktop 子结构。
// 语义（data-model §1.1）：Desktop==nil 表示「从未设置」→ 读侧给默认值；
// 非 nil 则为权威值（三个布尔均显式落盘，不用 omitempty——否则 trayResident=false
// 会被吞掉、读回归一成 true，显式关闭无法持久）。旧版 CLI 读新文件：未知字段忽略，天然兼容。

func desktopTestEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "providers.json")
	os.Setenv("CC_SELECT_CONFIG", cfg)
	t.Cleanup(func() { os.Unsetenv("CC_SELECT_CONFIG") })
	return dir
}

func TestDesktopPrefs_DefaultsWhenUnset(t *testing.T) {
	desktopTestEnv(t)

	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if p.Desktop != nil {
		t.Fatalf("Desktop should be nil when unset, got %+v", p.Desktop)
	}
	got := p.LoadDesktop()
	want := DesktopPrefs{TrayResident: true, AutoStart: false, AutoUpdateCheck: true}
	if got != want {
		t.Fatalf("LoadDesktop() = %+v, want %+v", got, want)
	}
}

func TestDesktopPrefs_RoundTripPreservesExplicitFalse(t *testing.T) {
	desktopTestEnv(t)

	in := &Prefs{Desktop: &DesktopPrefs{TrayResident: false, AutoStart: true, AutoUpdateCheck: false}}
	if err := Save(in); err != nil {
		t.Fatal(err)
	}
	out, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	got := out.LoadDesktop()
	want := DesktopPrefs{TrayResident: false, AutoStart: true, AutoUpdateCheck: false}
	if got != want {
		t.Fatalf("round-trip = %+v, want %+v（显式 false 不得被默认值吞掉）", got, want)
	}
}

func TestDesktopPrefs_LoadOldFormatWithoutDesktopField(t *testing.T) {
	dir := desktopTestEnv(t)
	// 旧版格式：只有既有字段（模拟旧 CLI 写出的 prefs.json）。
	_ = os.WriteFile(filepath.Join(dir, "prefs.json"),
		[]byte(`{"isolationMode":"settings-only","language":"zh","keychainEnabled":true}`), 0o600)

	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if p.IsolationMode != ModeSettingsOnly || p.Language != "zh" || !p.KeychainEnabled {
		t.Fatalf("既有字段读取被破坏: %+v", p)
	}
	if got := p.LoadDesktop(); got != (DesktopPrefs{TrayResident: true, AutoStart: false, AutoUpdateCheck: true}) {
		t.Fatalf("旧格式下桌面偏好应为默认值, got %+v", got)
	}
}
