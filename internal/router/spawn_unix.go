//go:build !windows

package router

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// spawnDetachedRouter 以分离进程拉起 daemon（`cc-select router serve --foreground`）。
// Unix：setsid 脱离会话组，父进程退出不影响 daemon；env 原样继承（CC_SELECT_CONFIG
// 等测试/定位变量随之传递）。os.Args[0] 在生产即 cc-select 二进制路径。
// SpawnDetached 以分离进程拉起 daemon（`cc-select router serve --foreground`）。
func SpawnDetached(addr string) error {
	cmd := exec.Command(os.Args[0], "router", "serve", "--foreground")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn %s: %w", os.Args[0], err)
	}
	_ = cmd.Process.Release()
	_ = addr // addr 由状态文件/ResolveAddr 决定，serve 自行解析（研究 D7）
	return nil
}
