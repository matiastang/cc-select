package secrets

import (
	"errors"
	"testing"
)

func TestFakeStore_Contract(t *testing.T) {
	s := NewFake()
	svc := ServiceFor("glm", "ANTHROPIC_AUTH_TOKEN")

	// 取不存在的 → ErrNotFound
	if _, err := s.Get(svc); !errors.Is(err, ErrNotFound) {
		t.Errorf("不存在应返回 ErrNotFound，got %v", err)
	}

	// Set → Get 往返
	if err := s.Set(svc, "sk-secret"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(svc)
	if err != nil || got != "sk-secret" {
		t.Errorf("往返 want sk-secret got %q err %v", got, err)
	}

	// Set 覆盖
	s.Set(svc, "sk-new")
	got, _ = s.Get(svc)
	if got != "sk-new" {
		t.Errorf("覆盖失败 got %q", got)
	}

	// Delete 幂等（删两次都不报错）
	if err := s.Delete(svc); err != nil {
		t.Errorf("Delete 报错: %v", err)
	}
	if err := s.Delete(svc); err != nil {
		t.Errorf("Delete 幂等失败: %v", err)
	}
	if _, err := s.Get(svc); !errors.Is(err, ErrNotFound) {
		t.Errorf("删除后应 NotFound，got %v", err)
	}
}

func TestServiceFor_NamingConvention(t *testing.T) {
	if got := ServiceFor("glm", "ANTHROPIC_AUTH_TOKEN"); got != "cc-select:glm:ANTHROPIC_AUTH_TOKEN" {
		t.Errorf("命名 want cc-select:glm:ANTHROPIC_AUTH_TOKEN got %s", got)
	}
}
