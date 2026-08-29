package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/config"
)

// T012：RefreshPicker 刷新原语（specs/002 contracts/profile-settings.md §3 / research D3）。
// 热切后把「发射 profile」的 settings.json 改写为新 provider 的清单（字段级合并 + 原子写）。

func refreshFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestRefreshPicker_UpdatesPickerAndModelPreservesFields(t *testing.T) {
	p := refreshFixture(t, `{"env":{"ANTHROPIC_BASE_URL":"http://127.0.0.1:48270"},"model":"glm-5.3[1m]",
		"permissions":{"allow":["foo"]},"modelPicker":{"replaceBuiltInOptions":true,"options":[{"model":"glm-5.3[1m]"}]}}`)
	plan := config.ModelPlanFromEnv(map[string]string{
		"ANTHROPIC_MODEL":               "mm-m2.7",
		"HAIKU":                         "ignored", // 非标准键不得进入
		"ANTHROPIC_DEFAULT_HAIKU_MODEL": "mm-flash",
	})

	if err := RefreshPicker(p, plan); err != nil {
		t.Fatalf("RefreshPicker: %v", err)
	}
	m := readJSON(t, p)

	opts := pickerOptions(t, m)
	if len(opts) != 2 || opts[0].(map[string]any)["model"] != "mm-m2.7" ||
		opts[1].(map[string]any)["model"] != "mm-flash" {
		t.Fatalf("modelPicker 应为新 provider 清单: %v", opts)
	}
	if m["model"] != "mm-m2.7" {
		t.Fatalf("model 应为新主模型: %v", m["model"])
	}
	// 字段级合并：env/permissions 原样保留（FR-004 / INV-4）。
	env, _ := m["env"].(map[string]any)
	if env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:48270" {
		t.Fatalf("env 不得被触碰: %v", env)
	}
	if _, ok := m["permissions"].(map[string]any); !ok {
		t.Fatalf("permissions 应保留: %v", m)
	}
}

// T012：空 plan（目标 provider 无模型变量）→ 清除注入键，显示回落内置目录。
func TestRefreshPicker_EmptyPlanRemovesInjection(t *testing.T) {
	p := refreshFixture(t, `{"model":"glm-5.3[1m]","modelPicker":{"replaceBuiltInOptions":true,"options":[{"model":"glm-5.3[1m]"}]},"keep":1}`)
	empty := config.ModelPlanFromEnv(map[string]string{"ANTHROPIC_BASE_URL": "https://x"})

	if err := RefreshPicker(p, empty); err != nil {
		t.Fatalf("RefreshPicker: %v", err)
	}
	m := readJSON(t, p)
	if _, exists := m["modelPicker"]; exists {
		t.Fatalf("空 plan 应移除 modelPicker: %v", m["modelPicker"])
	}
	if _, exists := m["model"]; exists {
		t.Fatalf("空 plan 应移除 model: %v", m["model"])
	}
	if m["keep"] != float64(1) {
		t.Fatalf("其他字段应保留: %v", m["keep"])
	}
}

// T020 评审 finding 4：plan 有清单但无主模型（只配 DEFAULT 槽位）时，残留的
// model 是上一个 provider 的 id——CC 会标记列表外的模型，请求体携带旧 id 落入
// 改写规则末路透传给不认识它的下游。刷新应删除 model。
func TestRefreshPicker_NoMainDeletesStaleModel(t *testing.T) {
	p := refreshFixture(t, `{"model":"old-provider-model","keep":1}`)
	plan := config.ModelPlanFromEnv(map[string]string{"ANTHROPIC_DEFAULT_SONNET_MODEL": "slot-sonnet"})

	if err := RefreshPicker(p, plan); err != nil {
		t.Fatalf("RefreshPicker: %v", err)
	}
	m := readJSON(t, p)
	if _, exists := m["model"]; exists {
		t.Fatalf("无主模型时应删除残留 model: %v", m["model"])
	}
	if len(pickerOptions(t, m)) != 1 {
		t.Fatalf("modelPicker 应仍有清单: %v", m)
	}
	if m["keep"] != float64(1) {
		t.Fatalf("其他字段应保留: %v", m["keep"])
	}
}

// T012：availableModels 追加（与注入同规则，research D5）。
func TestRefreshPicker_AppendsAvailableModels(t *testing.T) {
	p := refreshFixture(t, `{"availableModels":["sonnet"],"env":{}}`)
	plan := config.ModelPlanFromEnv(map[string]string{"ANTHROPIC_MODEL": "mm-m2.7"})
	if err := RefreshPicker(p, plan); err != nil {
		t.Fatalf("RefreshPicker: %v", err)
	}
	m := readJSON(t, p)
	am, _ := m["availableModels"].([]any)
	got := make([]string, 0, len(am))
	for _, v := range am {
		got = append(got, v.(string))
	}
	if strings.Join(got, ",") != "sonnet,mm-m2.7" {
		t.Fatalf("availableModels = %v", got)
	}
}

// T012：损坏/缺失文件 → 错误（调用方决定告警；路由切换不得因此失败）。
func TestRefreshPicker_Errors(t *testing.T) {
	plan := fullPlan()
	if err := RefreshPicker(refreshFixture(t, `{broken`), plan); err == nil {
		t.Fatal("损坏 JSON 应返回错误")
	}
	if err := RefreshPicker(filepath.Join(t.TempDir(), "nope.json"), plan); err == nil {
		t.Fatal("缺失文件应返回错误")
	}
}
