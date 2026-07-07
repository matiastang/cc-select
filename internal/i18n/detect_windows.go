//go:build windows

package i18n

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                     = windows.NewLazySystemDLL("kernel32.dll")
	procGetUserDefaultLocaleName = kernel32.NewProc("GetUserDefaultLocaleName")
)

func detectSystemLocale() Locale {
	var buf [85]uint16
	r0, _, _ := syscall.SyscallN(
		procGetUserDefaultLocaleName.Addr(),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	n := int(r0)
	if n <= 1 {
		return DefaultLocale
	}
	name := syscall.UTF16ToString(buf[:n])
	return NormalizeLocale(name)
}
