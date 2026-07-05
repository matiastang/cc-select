//go:build !windows

package i18n

import (
	"os"
	"strings"
)

func detectSystemLocale() Locale {
	for _, name := range []string{"LC_ALL", "LANG", "LANGUAGE"} {
		v := os.Getenv(name)
		if v == "" || v == "C" || v == "POSIX" {
			continue
		}
		if name == "LANGUAGE" {
			// LANGUAGE can be a colon-separated list.
			if i := strings.Index(v, ":"); i != -1 {
				v = v[:i]
			}
		}
		if l := NormalizeLocale(v); IsSupportedLocale(string(l)) {
			return l
		}
	}
	return DefaultLocale
}
