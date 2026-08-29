//go:build !windows

package routes

import (
	"os"
	"syscall"
)

// lockFile 对锁文件加独占锁（Unix：flock，阻塞等待）。
func lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// unlockFile 释放锁。
func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
