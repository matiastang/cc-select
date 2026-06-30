//go:build !windows

package profile

import "os"

// makeLink 在 Unix（macOS/Linux）创建指向 target 的符号链接（link → target）。
// 软链免特权，目录与文件同法。文件型链接即使 target 尚不存在（悬挂）也能创建——
// claude 写入会穿透创建目标文件。
func makeLink(target, link string, _ bool) error {
	return os.Symlink(target, link)
}
