//go:build windows

package routes

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockFile 对锁文件加独占锁（Windows：LockFileEx，阻塞等待）。
// x/sys/windows 已是间接依赖（go-keyring），Windows 专属文件引入不污染其他平台。
func lockFile(f *os.File) error {
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &windows.Overlapped{})
}

// unlockFile 释放锁。
func unlockFile(f *os.File) error {
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &windows.Overlapped{})
}
