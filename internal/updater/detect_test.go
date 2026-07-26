package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectManager(t *testing.T) {
	cases := []struct {
		path string
		want Manager
	}{
		{"/opt/homebrew/Cellar/cc-select/1.0.0/bin/cc-select", ManagerHomebrew},
		{"/usr/local/Cellar/cc-select/1.0.0/bin/cc-select", ManagerHomebrew},
		{`/opt/homebrew/bin/cc-select`, ManagerNone}, // 未解析符号链接时不是 Cellar——调用方负责 EvalSymlinks
		{`C:\Users\x\scoop\apps\cc-select\current\cc-select.exe`, ManagerScoop},
		{`C:/Users/X/Scoop/Apps/cc-select/current/cc-select.exe`, ManagerScoop}, // 大小写不敏感
		{"/usr/local/bin/cc-select", ManagerNone},
		{"/home/x/.local/bin/cc-select", ManagerNone},
		{`C:\Tools\cc-select.exe`, ManagerNone},
	}
	for _, c := range cases {
		if got := DetectManager(c.path); got != c.want {
			t.Errorf("DetectManager(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestIsWritable(t *testing.T) {
	if !IsWritable(t.TempDir()) {
		t.Error("temp 目录应可写")
	}
	if IsWritable(filepath.Join(t.TempDir(), "no-such-dir", "deeper")) {
		t.Error("不存在的目录应不可写")
	}
}

func TestDetectContext_SymlinkResolved(t *testing.T) {
	// 模拟 Homebrew 布局：Cellar 真身 + bin 软链。DetectContext 必须解析软链，
	// 否则 DetectManager 看不到 /cellar/。
	dir := t.TempDir()
	cellar := filepath.Join(dir, "Cellar", "cc-select", "1.0.0", "bin")
	if err := os.MkdirAll(cellar, 0o755); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(cellar, "cc-select")
	if err := os.WriteFile(real, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	linkDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(linkDir, "cc-select")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("无法创建符号链接（Windows 需开发者模式）: %v", err)
	}

	old := executableFn
	executableFn = func() (string, error) { return link, nil }
	t.Cleanup(func() { executableFn = old })

	ctx, err := DetectContext()
	if err != nil {
		t.Fatalf("DetectContext: %v", err)
	}
	if ctx.Manager != ManagerHomebrew {
		t.Errorf("软链解析后应识别 Homebrew, got %v (path=%s)", ctx.Manager, ctx.ExecPath)
	}
}
