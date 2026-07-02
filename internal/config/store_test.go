package config

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

// setTempConfig 把 CC_SELECT_CONFIG 指向一个临时路径，返回清理函数。
func setTempConfig(t *testing.T) (path string, cleanup func()) {
	t.Helper()
	dir := t.TempDir()
	path = filepath.Join(dir, "providers.json")
	old, had := os.LookupEnv("CC_SELECT_CONFIG")
	os.Setenv("CC_SELECT_CONFIG", path)
	return path, func() {
		if had {
			os.Setenv("CC_SELECT_CONFIG", old)
		} else {
			os.Unsetenv("CC_SELECT_CONFIG")
		}
	}
}

// wantFilePerm 返回当前平台下秘密文件的期望权限。
// Unix/macOS 为 0600；Windows 上 os.Chmod 只控制 read-only 位，Perm() 返回 0666。
func wantFilePerm() os.FileMode {
	if runtime.GOOS == "windows" {
		return 0o666
	}
	return 0o600
}

func TestLoad_MissingReturnsDefault(t *testing.T) {
	_, cleanup := setTempConfig(t)
	defer cleanup()

	c, err := Load()
	if err != nil {
		t.Fatalf("Load 缺文件不应报错: %v", err)
	}
	if _, ok := c.Providers[OfficialProviderID]; !ok {
		t.Fatalf("缺文件应返回含官方 provider 的默认配置，got %+v", c.Providers)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	path, cleanup := setTempConfig(t)
	defer cleanup()

	in := &Config{Providers: map[string]Provider{
		"glm": {ID: "glm", Name: "智谱 GLM", Env: map[string]string{
			"ANTHROPIC_BASE_URL": "https://glm.example/v1",
			"ANTHROPIC_MODEL":    "glm-4.6",
		}},
	}}
	if err := Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 权限应为 0600。
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != wantFilePerm() {
		t.Errorf("文件权限 want %#o got %#o", wantFilePerm(), perm)
	}

	out, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.Providers["glm"].Env["ANTHROPIC_MODEL"] != "glm-4.6" {
		t.Errorf("往返后 env 丢失: %+v", out.Providers["glm"])
	}
}

func TestSave_Atomic_NoTmpLeftover(t *testing.T) {
	_, cleanup := setTempConfig(t)
	defer cleanup()

	if err := Save(Default()); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(mustConfigPath(t))
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if name := e.Name(); len(name) > 4 && name[len(name)-4:] == ".tmp" {
			t.Errorf("原子写后残留临时文件: %s", name)
		}
	}
}

func TestUsedVars_DerivedDedup(t *testing.T) {
	c := &Config{Providers: map[string]Provider{
		"a": {ID: "a", Env: map[string]string{"X": "1", "Y": "2"}},
		"b": {ID: "b", Env: map[string]string{"Y": "3", "Z": "4"}}, // Y 重复
	}}
	got := c.UsedVars()
	sort.Strings(got)
	want := []string{"X", "Y", "Z"}
	if len(got) != len(want) {
		t.Fatalf("UsedVars want %v got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("UsedVars[%d] want %s got %s", i, want[i], got[i])
		}
	}
}

func TestKeychainPlaceholder(t *testing.T) {
	if !IsKeychainPlaceholder("$keychain:cc-select:glm:K") {
		t.Error("应识别占位")
	}
	if IsKeychainPlaceholder("sk-plain") {
		t.Error("不应把明文当占位")
	}
	if got := KeychainService("$keychain:cc-select:glm:K"); got != "cc-select:glm:K" {
		t.Errorf("service 解析 want cc-select:glm:K got %s", got)
	}
	if got := KeychainService("sk-plain"); got != "" {
		t.Errorf("明文 service 应为空 got %s", got)
	}
}

func mustConfigPath(t *testing.T) string {
	t.Helper()
	p, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	return p
}
