//go:build windows

package i18n

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func detectSystemLocale() Locale {
	var buf [85]uint16
	n, err := windows.GetUserDefaultLocaleName(&buf[0], uint32(len(buf)))
	if err != nil || n <= 1 {
		return DefaultLocale
	}
	name := syscall.UTF16ToString(buf[:n])
	return NormalizeLocale(name)
}
