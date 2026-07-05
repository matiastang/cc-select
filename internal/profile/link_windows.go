//go:build windows

package profile

import (
	"os"
	"os/exec"

	"github.com/cc-select/cc-select/internal/i18n"
)

// makeLink 在 Windows 创建指向 target 的链接（link → target）。
//
//   - 目录：用 junction（cmd /c mklink /J）。junction 无需管理员/开发者模式特权，
//     覆盖了 Mode B 要共享的绝大多数（目录型）条目。
//   - 文件：用 os.Symlink（符号链接），需开发者模式或管理员；失败则返回错误，
//     调用方按「尽力而为」跳过+告警（仅 history.json 等个别文件，影响很小）。
//
// 后续增强：文件型可改硬链接（mklink /H，免特权、同卷）以实现无特权文件共享。
func makeLink(target, link string, isDir bool) error {
	if isDir {
		// junction 目标须为绝对路径；target/link 由调用方保证绝对。
		cmd := exec.Command("cmd", "/c", "mklink", "/J", link, target)
		if out, err := cmd.CombinedOutput(); err != nil {
			return i18n.Ew("profile.createJunction", err, link, string(out))
		}
		return nil
	}
	if err := os.Symlink(target, link); err != nil {
		return i18n.Ew("profile.createSymlink", err, link)
	}
	return nil
}
