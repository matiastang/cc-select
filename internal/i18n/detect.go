package i18n

import (
	"os"
	"strings"
)

// IsSupportedLocale reports whether s is a supported locale code.
func IsSupportedLocale(s string) bool {
	switch Locale(s) {
	case EN, ZH:
		return true
	}
	return false
}

// NormalizeLocale normalizes a raw locale string (e.g. "zh_CN.UTF-8",
// "zh-CN", "en_GB") to a supported locale code. It returns an empty Locale
// when the input cannot be mapped to a supported locale.
func NormalizeLocale(raw string) Locale {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Drop encoding/variant suffix.
	if i := strings.IndexAny(raw, ".@"); i != -1 {
		raw = raw[:i]
	}
	low := strings.ToLower(raw)
	// Map known Chinese variants.
	switch low {
	case "zh", "zh_cn", "zh-cn", "zh_hans", "zh-hans", "zh_sg", "zh-sg", "cmn":
		return ZH
	case "en", "en_us", "en-us", "en_gb", "en-gb", "en_ca", "en-ca", "en_au", "en-au":
		return EN
	}
	// Generic prefix match.
	if strings.HasPrefix(low, "zh") {
		return ZH
	}
	if strings.HasPrefix(low, "en") {
		return EN
	}
	return ""
}

// ResolveLocale picks the effective locale using the priority:
//   1. CC_SELECT_LANGUAGE env var
//   2. prefsLang (user setting from prefs.json)
//   3. OS detection
//   4. Default English
func ResolveLocale(prefsLang string) Locale {
	if env := os.Getenv("CC_SELECT_LANGUAGE"); env != "" {
		if l := NormalizeLocale(env); l != "" {
			return l
		}
	}
	if prefsLang != "" {
		if l := NormalizeLocale(prefsLang); l != "" {
			return l
		}
	}
	if l := detectSystemLocale(); l != "" {
		return l
	}
	return DefaultLocale
}
