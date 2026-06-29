package app

import (
	"path/filepath"
	"testing"

	"github.com/cc-select/cc-select/internal/config"
)

func TestNew_AssemblesDeps(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CC_SELECT_CONFIG", filepath.Join(dir, "providers.json"))

	a, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if a.Config == nil {
		t.Error("App.Config 不应为 nil")
	}
	// 缺文件应返回含官方 provider 的默认配置。
	if _, ok := a.Config.Providers[config.OfficialProviderID]; !ok {
		t.Errorf("默认配置应含官方 provider: %+v", a.Config.Providers)
	}
	if a.Secrets == nil {
		t.Error("App.Secrets 不应为 nil")
	}
}
