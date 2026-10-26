//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// defaultInstallDirs windows 候选目录：CC_SELECT_INSTALL_DIR 显式覆盖优先，
// 默认用户级程序目录（无需管理员）。
func defaultInstallDirs() []string {
	if d := os.Getenv("CC_SELECT_INSTALL_DIR"); d != "" {
		return []string{d}
	}
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		if home, err := os.UserHomeDir(); err == nil {
			base = filepath.Join(home, "AppData", "Local")
		}
	}
	if base == "" {
		return []string{filepath.Join(os.TempDir(), "cc-select-install")}
	}
	return []string{filepath.Join(base, "Programs", "cc-select")}
}

// afterInstall 确保 dir 在用户 PATH（HKCU\Environment\Path，无需管理员）。
// 已存在（含尾部反斜杠变体）则不动；写入用 REG_EXPAND_SZ 保留 %VAR% 展开。
func afterInstall(dir string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	val, _, err := k.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return err
	}
	for _, p := range strings.Split(val, ";") {
		if strings.EqualFold(strings.TrimRight(p, `\`), strings.TrimRight(dir, `\`)) {
			return nil // 已在 PATH
		}
	}
	newVal := val
	if newVal != "" && !strings.HasSuffix(newVal, ";") {
		newVal += ";"
	}
	newVal += dir
	// 必须 REG_EXPAND_SZ：HKCU\Environment\Path 原生展开类型；SetStringValue 写
	// REG_SZ 会让既有 %VAR% 条目在下次登录不再展开（评审 MED-4）。
	return k.SetExpandStringValue("Path", newVal)
}
