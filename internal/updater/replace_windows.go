//go:build windows

package updater

import (
	"os"

	"github.com/cc-select/cc-select/internal/i18n"
)

type windowsReplacer struct{}

func defaultReplacer() Replacer { return windowsReplacer{} }

// Replace 移植 scripts/install.ps1:143-159 的 .old rename dance：
// Windows 不允许覆盖正在运行的 exe，但允许 rename 它——所以先把当前 exe
// rename 成 .old，再把新二进制移入原路径。运行中进程从 renamed-.old 的
// 映像继续服务，新版本在下次启动时生效。
func (windowsReplacer) Replace(newBin, target string) error {
	old := target + ".old"
	// 1. 删除上次 run 残留的 .old（当前进程若是它则删除失败——留给下次启动清理）。
	if _, err := os.Stat(old); err == nil {
		if err := os.Remove(old); err != nil {
			return i18n.Ew("errors.update.replaceFailed", err)
		}
	}
	// 2. 把运行中的 exe rename 成 .old。
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, old); err != nil {
			return i18n.Ew("errors.update.replaceFailed", err)
		}
	}
	// 3. 新二进制移入原路径（与 target 同目录，rename 原子）。
	if err := os.Rename(newBin, target); err != nil {
		return i18n.Ew("errors.update.replaceFailed", err)
	}
	return nil
}

// CleanupStaleBackup 删除上次 run 残留的 <target>.old。best-effort：
// 若 .old 正被当前运行进程持有（本进程刚被 rename 成 .old），删除会失败，
// 由调用方忽略错误，下次启动（进程已退出）即可清掉。
func (windowsReplacer) CleanupStaleBackup(target string) error {
	old := target + ".old"
	if _, err := os.Stat(old); err != nil {
		return nil
	}
	return os.Remove(old)
}
