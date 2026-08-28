package secrets

import (
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/config"
)

func TestMigrateAll_PlaintextToKeychain(t *testing.T) {
	store := NewFake()
	cfg := &config.Config{Providers: map[string]config.Provider{
		"glm": {ID: "glm", Env: map[string]string{
			"ANTHROPIC_BASE_URL":   "https://glm.api",
			"ANTHROPIC_AUTH_TOKEN": "sk-plain",
			"ANTHROPIC_API_KEY":    "sk-ak",
			"ANTHROPIC_MODEL":      "glm-5.1",
		}},
		"done": {ID: "done", Env: map[string]string{
			"ANTHROPIC_AUTH_TOKEN": "$keychain:cc-select:done:ANTHROPIC_AUTH_TOKEN",
		}},
	}}

	n, failed := MigrateAll(store, cfg)
	if len(failed) != 0 {
		t.Fatalf("不应有失败: %v", failed)
	}
	if n != 2 { // glm 的 TOKEN + API_KEY（done 已是占位跳过）
		t.Errorf("迁移数 want 2 got %d", n)
	}
	env := cfg.Providers["glm"].Env
	if env["ANTHROPIC_AUTH_TOKEN"] != "$keychain:cc-select:glm:ANTHROPIC_AUTH_TOKEN" {
		t.Errorf("TOKEN 应换占位: %q", env["ANTHROPIC_AUTH_TOKEN"])
	}
	if env["ANTHROPIC_API_KEY"] != "$keychain:cc-select:glm:ANTHROPIC_API_KEY" {
		t.Errorf("API_KEY 应换占位: %q", env["ANTHROPIC_API_KEY"])
	}
	// 非敏感值不动。
	if env["ANTHROPIC_BASE_URL"] != "https://glm.api" || env["ANTHROPIC_MODEL"] != "glm-5.1" {
		t.Errorf("非敏感值不得被改: %+v", env)
	}
	// keychain 收到真值。
	if v, _ := store.Get("cc-select:glm:ANTHROPIC_AUTH_TOKEN"); v != "sk-plain" {
		t.Errorf("keychain 应存真值: %q", v)
	}

	// 幂等：再跑一遍无新增。
	n2, failed2 := MigrateAll(store, cfg)
	if n2 != 0 || len(failed2) != 0 {
		t.Errorf("二次迁移应无操作: n=%d failed=%v", n2, failed2)
	}
}

// halfFailingStore 只对特定 service 失败。
type halfFailingStore struct{ inner SecretStore }

func (h halfFailingStore) Get(s string) (string, error) { return h.inner.Get(s) }
func (h halfFailingStore) Set(s, v string) error {
	if strings.Contains(s, "ANTHROPIC_API_KEY") {
		return errMigrateStub
	}
	return h.inner.Set(s, v)
}
func (h halfFailingStore) Delete(s string) error { return h.inner.Delete(s) }

var errMigrateStub = &migrateStubError{}

type migrateStubError struct{}

func (*migrateStubError) Error() string { return "locked" }

func TestMigrateAll_FailureKeepsPlaintextAndReports(t *testing.T) {
	store := halfFailingStore{inner: NewFake()}
	cfg := &config.Config{Providers: map[string]config.Provider{
		"glm": {ID: "glm", Env: map[string]string{
			"ANTHROPIC_AUTH_TOKEN": "sk-ok",
			"ANTHROPIC_API_KEY":    "sk-bad",
		}},
	}}
	n, failed := MigrateAll(store, cfg)
	if n != 1 {
		t.Errorf("成功迁移数 want 1 got %d", n)
	}
	if len(failed) != 1 || !strings.Contains(failed[0], "glm:ANTHROPIC_API_KEY") {
		t.Errorf("失败明细应点名条目: %v", failed)
	}
	env := cfg.Providers["glm"].Env
	if env["ANTHROPIC_API_KEY"] != "sk-bad" {
		t.Errorf("失败项应保持明文（不产生半迁移态）: %q", env["ANTHROPIC_API_KEY"])
	}
	if env["ANTHROPIC_AUTH_TOKEN"] == "sk-ok" {
		t.Error("成功项应已换占位")
	}
}
