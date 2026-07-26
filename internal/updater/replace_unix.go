//go:build !windows

package updater

import (
	"io"
	"os"

	"github.com/cc-select/cc-select/internal/i18n"
)

type unixReplacer struct{}

func defaultReplacer() Replacer { return unixReplacer{} }

// Replace 原子替换：先备份现有二进制到 <target>.bak（单槽覆盖，同 install.sh:197），
// 再 os.Rename 新二进制到位。newBin 与 target 同目录（extractBinary 保证同文件系统），
// rename 是原子操作；运行中进程持有旧 inode，继续服务不受影响。
func (unixReplacer) Replace(newBin, target string) error {
	if _, err := os.Stat(target); err == nil {
		bak := target + ".bak"
		_ = os.Remove(bak) // 覆盖上次备份
		if err := copyFile(target, bak); err != nil {
			return i18n.Ew("errors.update.replaceFailed", err)
		}
	}
	if err := os.Rename(newBin, target); err != nil {
		return i18n.Ew("errors.update.replaceFailed", err)
	}
	return nil
}

// CleanupStaleBackup 在 Unix 上是 no-op：.bak 被刻意保留作为回滚安全网。
func (unixReplacer) CleanupStaleBackup(string) error { return nil }

// copyFile 复制文件内容并赋予可执行权限（用于 .bak 备份）。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
