package router

import (
	"fmt"
	"testing"

	"github.com/cc-select/cc-select/internal/secrets"
)

// countingStore 包装 FakeStore 统计 Get 次数（验证内存缓存命中）。
type countingStore struct {
	inner *secrets.FakeStore
	gets  int
}

func (c *countingStore) Get(service string) (string, error) {
	c.gets++
	return c.inner.Get(service)
}
func (c *countingStore) Set(service, value string) error { return c.inner.Set(service, value) }
func (c *countingStore) Delete(service string) error     { return c.inner.Delete(service) }

func TestResolve_PlainValuePassesThrough(t *testing.T) {
	r := NewKeychainResolver(secrets.NewFake())
	for _, v := range []string{"", "https://api.example.com", "glm-5.1", "$notkeychain:x"} {
		got, err := r.Resolve(v)
		if err != nil || got != v {
			t.Errorf("非占位应原样返回 %q: got %q err=%v", v, got, err)
		}
	}
}

func TestResolve_PlaceholderFromKeychain(t *testing.T) {
	store := secrets.NewFake()
	_ = store.Set("cc-select:glm:ANTHROPIC_AUTH_TOKEN", "sk-real")
	r := NewKeychainResolver(store)

	got, err := r.Resolve("$keychain:cc-select:glm:ANTHROPIC_AUTH_TOKEN")
	if err != nil || got != "sk-real" {
		t.Errorf("占位应解析为真值: got %q err=%v", got, err)
	}
}

func TestResolve_CacheAvoidsRepeatedKeychainAccess(t *testing.T) {
	store := &countingStore{inner: secrets.NewFake()}
	_ = store.Set("cc-select:glm:ANTHROPIC_AUTH_TOKEN", "sk-real")
	r := NewKeychainResolver(store)

	ph := "$keychain:cc-select:glm:ANTHROPIC_AUTH_TOKEN"
	for range 5 {
		if _, err := r.Resolve(ph); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
	}
	if store.gets != 1 {
		t.Errorf("内存缓存应只访问 keychain 一次，got %d", store.gets)
	}
}

// failingStore 模拟 keychain 故障。
type failingStore struct{}

func (failingStore) Get(string) (string, error) { return "", fmt.Errorf("keychain locked") }
func (failingStore) Set(string, string) error   { return nil }
func (failingStore) Delete(string) error        { return nil }

func TestResolve_StoreErrorIsDiagnostic(t *testing.T) {
	r := NewKeychainResolver(failingStore{})
	_, err := r.Resolve("$keychain:cc-select:glm:ANTHROPIC_AUTH_TOKEN")
	if err == nil {
		t.Fatal("keychain 取值失败应报错")
	}
	// 可诊断：错误应点名 service，方便用户定位是哪条密钥取不到。
	if !containsStr(err.Error(), "cc-select:glm:ANTHROPIC_AUTH_TOKEN") {
		t.Errorf("错误应含 service 名: %v", err)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && indexStr(s, sub) >= 0
}

func indexStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
