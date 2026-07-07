//go:build windows

package i18n

import (
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procGetUserDefaultLocaleName = kernel32.NewProc("GetUserDefaultLocaleName")
)

func detectSystemLocale() Locale {
	// Respect Unix-style locale env vars when explicitly set (e.g. in tests or
	// Git Bash/WSL environments), then fall back to the Win32 API.
	for _, name := range []string{"LC_ALL", "LANG", "LANGUAGE"} {
		v := os.Getenv(name)
		if v == "" || v == "C" || v == "POSIX" {
			continue
		}
		if name == "LANGUAGE" {
			if i := strings.Index(v, ":"); i != -1 {
				v = v[:i]
			}
		}
		if l := NormalizeLocale(v); IsSupportedLocale(string(l)) {
			return l
		}
	}

	var buf [85]uint16
	r, _, _ := procGetUserDefaultLocaleName.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if r == 0 {
		return DefaultLocale
	}
	name := syscall.UTF16ToString(buf[:int(r)])
	if l := NormalizeLocale(name); IsSupportedLocale(string(l)) {
		return l
	}
	return DefaultLocale
}
