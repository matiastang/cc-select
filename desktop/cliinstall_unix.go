//go:build !windows

package main

import (
	"os"
	"path/filepath"
)

// defaultInstallDirs unix 候选目录：/usr/local/bin 优先，无权限回落 ~/.bin
// （data-model §1.2 / contracts §3.3）。
func defaultInstallDirs() []string {
	dirs := []string{"/usr/local/bin"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dirs = append(dirs, filepath.Join(home, ".bin"))
	}
	return dirs
}

// afterInstall unix 侧无需额外动作（目标目录默认在 PATH；回落场景由 hint 提示）。
func afterInstall(dir string) error { return nil }
