package i18n

import (
	"os"
	"testing"
)

func TestBundle_T_ActiveLocale(t *testing.T) {
	b := New(map[string]map[Locale]string{
		"greeting": {EN: "hello", ZH: "你好"},
	})
	b.SetLocale(ZH)
	if got := b.T("greeting"); got != "你好" {
		t.Errorf("ZH greeting = %q, want 你好", got)
	}
}

func TestBundle_T_FallbackToEnglish(t *testing.T) {
	b := New(map[string]map[Locale]string{
		"only_en": {EN: "english only"},
	})
	b.SetLocale(ZH)
	if got := b.T("only_en"); got != "english only" {
		t.Errorf("fallback = %q, want english only", got)
	}
}

func TestBundle_T_FallbackToKey(t *testing.T) {
	b := New(map[string]map[Locale]string{})
	b.SetLocale(EN)
	if got := b.T("missing"); got != "missing" {
		t.Errorf("key fallback = %q, want missing", got)
	}
}

func TestBundle_T_FormatArgs(t *testing.T) {
	b := New(map[string]map[Locale]string{
		"hello": {EN: "hello %s", ZH: "你好 %s"},
	})
	b.SetLocale(EN)
	if got := b.T("hello", "world"); got != "hello world" {
		t.Errorf("formatted = %q, want hello world", got)
	}
}

func TestIsSupportedLocale(t *testing.T) {
	if !IsSupportedLocale("en") {
		t.Error("en should be supported")
	}
	if !IsSupportedLocale("zh") {
		t.Error("zh should be supported")
	}
	if IsSupportedLocale("fr") {
		t.Error("fr should not be supported")
	}
}

func TestNormalizeLocale(t *testing.T) {
	cases := []struct {
		in   string
		want Locale
	}{
		{"zh_CN.UTF-8", ZH},
		{"zh-CN", ZH},
		{"zh_Hans", ZH},
		{"zh", ZH},
		{"en_US.UTF-8", EN},
		{"en-GB", EN},
		{"en", EN},
		{"fr", ""},
		{"C", ""},
		{"POSIX", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := NormalizeLocale(c.in); got != c.want {
			t.Errorf("NormalizeLocale(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestResolveLocale_Priority(t *testing.T) {
	t.Setenv("CC_SELECT_LANGUAGE", "")
	os.Unsetenv("LC_ALL")
	os.Unsetenv("LANG")
	os.Unsetenv("LANGUAGE")

	// Default -> OS detection, or EN fallback if detection returns empty.
	wantDefault := detectSystemLocale()
	if wantDefault == "" {
		wantDefault = DefaultLocale
	}
	if got := ResolveLocale(""); got != wantDefault {
		t.Errorf("empty = %q, want %q", got, wantDefault)
	}

	// Prefs wins over default.
	if got := ResolveLocale("zh"); got != ZH {
		t.Errorf("prefs = %q, want zh", got)
	}

	// Env wins over prefs.
	t.Setenv("CC_SELECT_LANGUAGE", "en")
	if got := ResolveLocale("zh"); got != EN {
		t.Errorf("env = %q, want en", got)
	}

	// Env invalid -> prefs.
	t.Setenv("CC_SELECT_LANGUAGE", "fr")
	if got := ResolveLocale("zh"); got != ZH {
		t.Errorf("env invalid = %q, want zh", got)
	}
}

func TestDetectSystemLocale_RespectsEnv(t *testing.T) {
	// Save and restore.
	oldLC := os.Getenv("LC_ALL")
	oldLang := os.Getenv("LANG")
	oldLangs := os.Getenv("LANGUAGE")
	defer func() {
		os.Setenv("LC_ALL", oldLC)
		os.Setenv("LANG", oldLang)
		os.Setenv("LANGUAGE", oldLangs)
	}()

	os.Unsetenv("LC_ALL")
	os.Unsetenv("LANGUAGE")
	os.Setenv("LANG", "zh_CN.UTF-8")

	if got := detectSystemLocale(); got != ZH {
		t.Errorf("LANG=zh_CN = %q, want zh", got)
	}
}

func TestDefaultBundle(t *testing.T) {
	// The default bundle is loaded from embedded JSON; sanity-check a known key.
	if got := T("cli.list.header.id"); got != "ID" {
		t.Errorf("default bundle cli.list.header.id = %q, want ID", got)
	}
	SetLocale(ZH)
	t.Cleanup(func() { SetLocale(EN) })
	if got := T("cli.list.header.id"); got != "ID" {
		t.Errorf("zh cli.list.header.id = %q, want ID", got)
	}
}

// TestLocaleKeyParity verifies that every translation key present in en.json
// also exists in zh.json and vice versa.
func TestLocaleKeyParity(t *testing.T) {
	catalog, err := loadCatalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	enKeys := map[string]struct{}{}
	zhKeys := map[string]struct{}{}
	for key, locs := range catalog {
		if _, ok := locs[EN]; ok {
			enKeys[key] = struct{}{}
		}
		if _, ok := locs[ZH]; ok {
			zhKeys[key] = struct{}{}
		}
	}
	for key := range enKeys {
		if _, ok := zhKeys[key]; !ok {
			t.Errorf("key %q present in en.json but missing in zh.json", key)
		}
	}
	for key := range zhKeys {
		if _, ok := enKeys[key]; !ok {
			t.Errorf("key %q present in zh.json but missing in en.json", key)
		}
	}
}
