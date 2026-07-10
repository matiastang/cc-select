package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/config"
)

// setTempRoot 把 CC_SELECT_CONFIG 指向临时目录，使 profiles 也落在该目录下（隔离）。
func setTempRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "providers.json")
	t.Setenv("CC_SELECT_CONFIG", cfgPath)
	return dir
}

// wantFilePerm 返回当前平台下秘密文件的期望权限。
// Unix/macOS 为 0600；Windows 上 os.Chmod 只控制 read-only 位，Perm() 返回 0666。
func wantFilePerm() os.FileMode {
	if runtime.GOOS == "windows" {
		return 0o666
	}
	return 0o600
}

func TestEnsure_WritesSettingsJSON(t *testing.T) {
	setTempRoot(t)
	env := map[string]string{
		"ANTHROPIC_BASE_URL":   "https://api.minimaxi.com/anthropic",
		"ANTHROPIC_AUTH_TOKEN": "sk-secret",
	}
	dir, err := Ensure("minimax", env)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	p := filepath.Join(dir, "settings.json")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// 内容含 env 键值。
	body := string(data)
	if !strings.Contains(body, "https://api.minimaxi.com/anthropic") || !strings.Contains(body, "sk-secret") {
		t.Errorf("settings.json 应含 env 值：%s", body)
	}
	// 权限：Unix/macOS 0600，Windows 0666（Chmod 仅控制 read-only）。
	info, _ := os.Stat(p)
	if perm := info.Mode().Perm(); perm != wantFilePerm() {
		t.Errorf("文件权限 want %#o got %#o", wantFilePerm(), perm)
	}
}

func TestEnsure_OfficialProviderNoop(t *testing.T) {
	setTempRoot(t)
	dir, err := Ensure(config.OfficialProviderID, map[string]string{"X": "1"})
	if err != nil {
		t.Fatalf("Ensure 官方: %v", err)
	}
	if dir != "" {
		t.Errorf("官方 Ensure 应返回空目录，got %q", dir)
	}
	exists, _ := Exists(config.OfficialProviderID)
	if exists {
		t.Error("官方不应有 profile 目录")
	}
}

func TestRemove_Idempotent(t *testing.T) {
	setTempRoot(t)
	Ensure("x", map[string]string{"X": "1"})
	if err := Remove("x"); err != nil {
		t.Errorf("第一次 Remove: %v", err)
	}
	if err := Remove("x"); err != nil { // 已删，再删不报错
		t.Errorf("第二次 Remove 应幂等: %v", err)
	}
	exists, _ := Exists("x")
	if exists {
		t.Error("Remove 后应不存在")
	}
}

func TestRemove_OfficialNoop(t *testing.T) {
	setTempRoot(t)
	if err := Remove(config.OfficialProviderID); err != nil {
		t.Errorf("官方 Remove 应 no-op: %v", err)
	}
}

func TestReadEnv_RoundTrip(t *testing.T) {
	setTempRoot(t)
	env := map[string]string{"ANTHROPIC_MODEL": "m1", "ANTHROPIC_BASE_URL": "u1"}
	if _, err := Ensure("glm", env); err != nil {
		t.Fatal(err)
	}
	got, err := ReadEnv("glm")
	if err != nil {
		t.Fatal(err)
	}
	if got["ANTHROPIC_MODEL"] != "m1" || got["ANTHROPIC_BASE_URL"] != "u1" {
		t.Errorf("ReadEnv 往返失败: %v", got)
	}
}

func TestReadEnv_MissingReturnsEmpty(t *testing.T) {
	setTempRoot(t)
	got, err := ReadEnv("nonexistent")
	if err != nil {
		t.Fatalf("缺失应返回空 map + nil err: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("缺失应返回空 map: %v", got)
	}
}

func TestDir_RootFollowsConfigPath(t *testing.T) {
	dir := setTempRoot(t)
	d, err := Dir("x")
	if err != nil {
		t.Fatal(err)
	}
	// profiles 应在 CC_SELECT_CONFIG 同级目录下。
	expected := filepath.Join(filepath.Dir(filepath.Join(dir, "providers.json")), "profiles", "x")
	if d != expected {
		t.Errorf("Dir want %s got %s", expected, d)
	}
}
