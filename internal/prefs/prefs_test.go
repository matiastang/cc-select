package prefs

import (
	"path/filepath"
	"testing"
)

func setTempPrefs(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	// CC_SELECT_CONFIG 指向 providers.json，prefs.json 落其同级。
	t.Setenv("CC_SELECT_CONFIG", filepath.Join(dir, "providers.json"))
}

func TestLoad_MissingReturnsEmpty(t *testing.T) {
	setTempPrefs(t)
	pr, err := Load()
	if err != nil {
		t.Fatalf("缺文件应返回空 Prefs + nil: %v", err)
	}
	if pr.IsolationMode != "" {
		t.Errorf("缺文件 IsolationMode 应为空，got %q", pr.IsolationMode)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	setTempPrefs(t)
	if err := Save(&Prefs{IsolationMode: ModeFull}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	pr, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if pr.IsolationMode != ModeFull {
		t.Errorf("往返失败 want %q got %q", ModeFull, pr.IsolationMode)
	}
}

func TestMode_Valid(t *testing.T) {
	if !(Mode("").Valid() && ModeSettingsOnly.Valid() && ModeFull.Valid()) {
		t.Error("空/settings-only/full 应合法")
	}
	if Mode("bogus").Valid() {
		t.Error("bogus 应非法")
	}
}

func TestResolveMode_TieredFallback(t *testing.T) {
	cases := []struct {
		name             string
		oneOff, prov, gl Mode
		want             Mode
	}{
		{"all empty → default", "", "", "", DefaultMode},
		{"oneOff wins", ModeFull, ModeSettingsOnly, ModeSettingsOnly, ModeFull},
		{"provider beats global", "", ModeFull, ModeSettingsOnly, ModeFull},
		{"global when no provider", "", "", ModeFull, ModeFull},
		{"settings-only explicit global", "", "", ModeSettingsOnly, ModeSettingsOnly},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ResolveMode(c.oneOff, c.prov, c.gl); got != c.want {
				t.Errorf("ResolveMode(%q,%q,%q) want %q got %q", c.oneOff, c.prov, c.gl, c.want, got)
			}
		})
	}
}
