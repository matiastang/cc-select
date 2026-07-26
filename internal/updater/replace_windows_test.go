//go:build windows

package updater

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWindowsReplacer_OldDance 验证 .old rename dance：
// target.exe（模拟运行中的旧 exe）→ Replace 后 target 内容为新、target.old 内容为旧。
// 纯文件操作（temp stub，非真实运行中的 exe），在 make test 即跑。
func TestWindowsReplacer_OldDance(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "cc-select.exe")
	newBin := filepath.Join(dir, "new.exe")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newBin, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}

	r := windowsReplacer{}
	if err := r.Replace(newBin, target); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	got, _ := os.ReadFile(target)
	if string(got) != "new" {
		t.Errorf("target = %q, want new", got)
	}
	old, err := os.ReadFile(target + ".old")
	if err != nil || string(old) != "old" {
		t.Errorf(".old = (%q,%v), want (old,nil)", old, err)
	}

	// CleanupStaleBackup 应清掉 .old。
	if err := r.CleanupStaleBackup(target); err != nil {
		t.Fatalf("CleanupStaleBackup: %v", err)
	}
	if _, err := os.Stat(target + ".old"); !os.IsNotExist(err) {
		t.Error(".old 应已被清理")
	}
}
