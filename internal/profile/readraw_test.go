package profile

import (
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/config"
)

func TestReadRaw_RoundTrip(t *testing.T) {
	setTempRoot(t)
	// 写入含 env 之外字段（model）的完整 settings.json。
	if _, err := EnsureRaw("x", []byte(`{"env":{"ANTHROPIC_BASE_URL":"https://x"},"model":"opus"}`)); err != nil {
		t.Fatalf("EnsureRaw: %v", err)
	}
	raw, err := ReadRaw("x")
	if err != nil {
		t.Fatalf("ReadRaw: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, `"model"`) || !strings.Contains(body, "https://x") {
		t.Errorf("ReadRaw 应返回磁盘原文（含 model 与 base url）: %s", body)
	}
}

func TestReadRaw_MissingReturnsNil(t *testing.T) {
	setTempRoot(t)
	raw, err := ReadRaw("nonexistent")
	if err != nil {
		t.Fatalf("缺失文件应返回 (nil, nil): %v", err)
	}
	if raw != nil {
		t.Errorf("缺失文件 raw 应为 nil，got %q", raw)
	}
}

func TestReadRaw_OfficialReturnsNil(t *testing.T) {
	setTempRoot(t)
	raw, err := ReadRaw(config.OfficialProviderID)
	if err != nil {
		t.Fatalf("官方 provider 应返回 (nil, nil): %v", err)
	}
	if raw != nil {
		t.Errorf("官方 provider raw 应为 nil，got %q", raw)
	}
}

func TestEnsureRaw_OfficialNoop(t *testing.T) {
	setTempRoot(t)
	dir, err := EnsureRaw(config.OfficialProviderID, []byte(`{"model":"x"}`))
	if err != nil {
		t.Fatalf("官方 EnsureRaw 应 no-op: %v", err)
	}
	if dir != "" {
		t.Errorf("官方 EnsureRaw 应返回空目录，got %q", dir)
	}
	if ok, _ := Exists(config.OfficialProviderID); ok {
		t.Error("官方 provider 不应落盘")
	}
}

// TestEnsureRaw_AppendsNewline 验证写入内容统一以换行结尾（便于 diff）。
func TestEnsureRaw_AppendsNewline(t *testing.T) {
	setTempRoot(t)
	if _, err := EnsureRaw("y", []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	raw, _ := ReadRaw("y")
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Errorf("EnsureRaw 应以换行结尾，got %q", raw)
	}
}

// TestDir_RejectsTraversal 验证非法 id（路径穿越）被 Dir 拒绝，
// 防止 Ensure/Remove 等写入或删除 profiles 根目录之外。
func TestDir_RejectsTraversal(t *testing.T) {
	setTempRoot(t)
	for _, id := range []string{"../evil", "..", "a/b", "../../tmp/x"} {
		if _, err := Dir(id); err == nil {
			t.Errorf("Dir(%q) 应拒绝路径穿越", id)
		}
		// EnsureRaw 经由 Dir 同样应被挡住。
		if _, err := EnsureRaw(id, []byte(`{}`)); err == nil {
			t.Errorf("EnsureRaw(%q) 应拒绝路径穿越", id)
		}
	}
}
