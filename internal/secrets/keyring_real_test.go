package secrets

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

// TestKeyringStore_Contract 用 go-keyring 的内存 Mock 后端覆盖真实 KeyringStore，
// 不触碰系统 Keychain（CI 友好），但走的是生产实现 New()/Get/Set/Delete。
func TestKeyringStore_Contract(t *testing.T) {
	keyring.MockInit() // 全局切到内存后端
	s := New()
	svc := ServiceFor("glm", "ANTHROPIC_AUTH_TOKEN")

	// 不存在 → 包内 sentinel ErrNotFound（而非 go-keyring 的 ErrNotFound）。
	if _, err := s.Get(svc); !errors.Is(err, ErrNotFound) {
		t.Errorf("不存在应返回 ErrNotFound，got %v", err)
	}

	// Set → Get 往返。
	if err := s.Set(svc, "sk-secret"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get(svc)
	if err != nil || got != "sk-secret" {
		t.Errorf("往返 want sk-secret got %q err %v", got, err)
	}

	// Set 覆盖。
	if err := s.Set(svc, "sk-new"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(svc); got != "sk-new" {
		t.Errorf("覆盖失败 got %q", got)
	}

	// Delete 幂等（删两次都不报错）。
	if err := s.Delete(svc); err != nil {
		t.Errorf("Delete: %v", err)
	}
	if err := s.Delete(svc); err != nil {
		t.Errorf("Delete 幂等失败: %v", err)
	}
	if _, err := s.Get(svc); !errors.Is(err, ErrNotFound) {
		t.Errorf("删除后应 NotFound，got %v", err)
	}
}
