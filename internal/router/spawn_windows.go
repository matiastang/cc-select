//go:build windows

package router

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Windows 进程创建标志：DETACHED_PROCESS（脱离控制台）| CREATE_NEW_PROCESS_GROUP。
const detachedFlags = 0x00000008 | 0x00000200

// spawnDetachedRouter 以分离进程拉起 daemon（`cc-select router serve --foreground`）。
// env 原样继承（CC_SELECT_CONFIG 等测试/定位变量随之传递）。
// os.Args[0] 在生产即 cc-select.exe 路径。
// SpawnDetached 以分离进程拉起 daemon（`cc-select router serve --foreground`）。
func SpawnDetached(addr string) error {
	cmd := exec.Command(os.Args[0], "router", "serve", "--foreground")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedFlags}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn %s: %w", os.Args[0], err)
	}
	_ = cmd.Process.Release()
	_ = addr // addr 由状态文件/ResolveAddr 决定，serve 自行解析（研究 D7）
	return nil
}
