package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/config"
)

// 注入形状与边界全集（specs/002 contracts/profile-settings.md INV-1~5 / research D5/D7/D8）。

func mustInject(t *testing.T, data string, plan config.ModelPlan, withModel bool) map[string]any {
	t.Helper()
	out, err := injectModelPicker([]byte(data), plan, withModel)
	if err != nil {
		t.Fatalf("injectModelPicker: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("输出非法 JSON: %v", err)
	}
	return m
}

func fullPlan() config.ModelPlan {
	return config.ModelPlanFromEnv(map[string]string{
		"ANTHROPIC_MODEL":               "glm-5.3[1m]",
		"ANTHROPIC_DEFAULT_OPUS_MODEL":  "glm-5.2[1m]",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL": "glm-5.3-Flash",
	})
}

func pickerOptions(t *testing.T, m map[string]any) []any {
	t.Helper()
	mp, ok := m["modelPicker"].(map[string]any)
	if !ok {
		t.Fatalf("modelPicker 缺失或类型错误: %v", m["modelPicker"])
	}
	if mp["replaceBuiltInOptions"] != true {
		t.Fatalf("replaceBuiltInOptions 应为 true: %v", mp["replaceBuiltInOptions"])
	}
	opts, ok := mp["options"].([]any)
	if !ok {
		t.Fatalf("options 缺失或类型错误: %v", mp["options"])
	}
	return opts
}

// T005：注入形状——replaceBuiltInOptions=true、options 仅含 model 键、顺序 = Entries。
func TestInjectModelPicker_Shape(t *testing.T) {
	m := mustInject(t, `{"env":{}}`, fullPlan(), false)
	opts := pickerOptions(t, m)
	want := []string{"glm-5.3[1m]", "glm-5.2[1m]", "glm-5.3-Flash"}
	if len(opts) != len(want) {
		t.Fatalf("options 数量 = %d, want %d", len(opts), len(want))
	}
	for i, w := range want {
		row, ok := opts[i].(map[string]any)
		if !ok {
			t.Fatalf("options[%d] 不是 object: %v", i, opts[i])
		}
		if len(row) != 1 {
			t.Fatalf("options[%d] 应只含 model 键（无 label/description）: %v", i, row)
		}
		if row["model"] != w {
			t.Errorf("options[%d].model = %v, want %s", i, row["model"], w)
		}
	}
}

// T006-1：空 plan → 零注入（键不存在，INV-5）。
func TestInjectModelPicker_EmptyPlanNoInjection(t *testing.T) {
	empty := config.ModelPlanFromEnv(map[string]string{"ANTHROPIC_BASE_URL": "https://x"})
	m := mustInject(t, `{"env":{},"permissions":{}}`, empty, true)
	if _, exists := m["modelPicker"]; exists {
		t.Fatalf("空 plan 不得注入 modelPicker: %v", m["modelPicker"])
	}
	if _, exists := m["model"]; exists {
		t.Fatalf("空 plan 不得注入 model: %v", m["model"])
	}
}

// T006-2：model 字段注入仅 withModel（Mode P）且仅取主模型（research D8）。
func TestInjectModelPicker_ModelField(t *testing.T) {
	plan := fullPlan()
	withM := mustInject(t, `{"env":{}}`, plan, true)
	if withM["model"] != "glm-5.3[1m]" {
		t.Fatalf("Mode P 应注入 model=主模型: %v", withM["model"])
	}
	withoutM := mustInject(t, `{"model":"opus[1m]","env":{}}`, plan, false)
	if withoutM["model"] != "opus[1m]" {
		t.Fatalf("Mode B 不得触碰 model 字段（env 优先 sufficient）: %v", withoutM["model"])
	}
}

// T006-3：未知字段全保留（FR-004 字段级合并语义）。
func TestInjectModelPicker_PreservesUnknownFields(t *testing.T) {
	m := mustInject(t, `{"permissions":{"allow":["Bash"]},"hooks":{"x":1},"custom":"keep","env":{}}`, fullPlan(), true)
	if m["custom"] != "keep" {
		t.Fatalf("custom 字段丢失: %v", m["custom"])
	}
	if _, ok := m["permissions"].(map[string]any); !ok {
		t.Fatalf("permissions 字段丢失: %v", m["permissions"])
	}
	if _, ok := m["hooks"].(map[string]any); !ok {
		t.Fatalf("hooks 字段丢失: %v", m["hooks"])
	}
}

// T006-4：availableModels 存在时追加缺失 id 且幂等（research D5 / FR-006）。
func TestInjectModelPicker_AppendsAvailableModels(t *testing.T) {
	in := `{"availableModels":["sonnet"],"env":{}}`
	m := mustInject(t, in, fullPlan(), false)
	am, ok := m["availableModels"].([]any)
	if !ok {
		t.Fatalf("availableModels 应保留: %v", m["availableModels"])
	}
	got := make([]string, 0, len(am))
	for _, v := range am {
		got = append(got, v.(string))
	}
	want := []string{"sonnet", "glm-5.3[1m]", "glm-5.2[1m]", "glm-5.3-Flash"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("availableModels = %v, want %v", got, want)
	}
	// 幂等：对已追加结果再注入，不产生重复。
	again, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	m2 := mustInject(t, string(again), fullPlan(), false)
	am2 := m2["availableModels"].([]any)
	if len(am2) != len(want) {
		t.Fatalf("幂等破坏: %v", am2)
	}
}

// T006-5：INV-1——注入内容零 keychain 占位。
func TestInjectModelPicker_NoSecrets(t *testing.T) {
	plan := config.ModelPlanFromEnv(map[string]string{
		"ANTHROPIC_MODEL":               "$keychain:cc-select:glm:ANTHROPIC_MODEL",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL": "glm-5.3-Flash",
		"ANTHROPIC_AUTH_TOKEN":          "$keychain:cc-select:glm:ANTHROPIC_AUTH_TOKEN",
	})
	m := mustInject(t, `{"env":{}}`, plan, true)
	for _, row := range pickerOptions(t, m) {
		if s := row.(map[string]any)["model"].(string); strings.Contains(s, "$keychain:") {
			t.Fatalf("占位泄漏进注入内容: %s", s)
		}
	}
}

// T006-6：输入非法 JSON → 返回错误（调用方决定降级/告警）。
func TestInjectModelPicker_InvalidJSON(t *testing.T) {
	if _, err := injectModelPicker([]byte(`{broken`), fullPlan(), false); err == nil {
		t.Fatal("非法 JSON 应返回错误")
	}
}

// T007 前置：buildPickerBlock 形状（供 US3 刷新复用的共享原语）。
func TestBuildPickerBlock(t *testing.T) {
	block := buildPickerBlock(fullPlan())
	if block["replaceBuiltInOptions"] != true {
		t.Fatalf("replaceBuiltInOptions: %v", block["replaceBuiltInOptions"])
	}
	opts, ok := block["options"].([]map[string]string)
	if !ok || len(opts) != 3 {
		t.Fatalf("options: %v", block["options"])
	}
	if opts[0]["model"] != "glm-5.3[1m]" {
		t.Fatalf("首行应为 main: %v", opts[0])
	}
}

// T008：SyncProxy 端到端——providers.json 真值 env 派生注入（modelPicker + Mode P 的 model），
// 且 profile env 仍仅含代理 BASE_URL（research D2/D8）。
func TestSyncProxy_InjectsPickerFromProviderEnv(t *testing.T) {
	setTempRoot(t)
	home := setTempClaudeHome(t)
	os.MkdirAll(filepath.Join(home, "projects"), 0o700)
	os.WriteFile(filepath.Join(home, "settings.json"),
		[]byte(`{"permissions":{"allow":["foo"]},"model":"opus[1m]"}`), 0o600)

	cfg := config.Default()
	cfg.Providers["glm"] = config.Provider{
		ID: "glm",
		Env: map[string]string{
			"ANTHROPIC_BASE_URL":            "https://open.bigmodel.cn/api/anthropic",
			"ANTHROPIC_AUTH_TOKEN":          "sk-x",
			"ANTHROPIC_MODEL":               "glm-5.3[1m]",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL": "glm-5.3-Flash",
		},
	}
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	dir, _, err := SyncProxy("glm", "http://127.0.0.1:48270")
	if err != nil {
		t.Fatalf("SyncProxy: %v", err)
	}
	m := map[string]any{}
	if err := json.Unmarshal([]byte(readProfileSettings(t, dir)), &m); err != nil {
		t.Fatal(err)
	}

	// env 仅含代理 BASE_URL（Mode P 恒定项，密钥不落 profile——宪法 II）。
	env, _ := m["env"].(map[string]any)
	if len(env) != 1 || env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:48270" {
		t.Fatalf("env 应仅含代理 BASE_URL: %v", env)
	}
	// modelPicker 由 providers.json 真值派生。
	opts := pickerOptions(t, m)
	if len(opts) != 2 || opts[0].(map[string]any)["model"] != "glm-5.3[1m]" ||
		opts[1].(map[string]any)["model"] != "glm-5.3-Flash" {
		t.Fatalf("modelPicker 注入错误: %v", opts)
	}
	// Mode P：model 覆盖全局继承的 opus[1m]（research D8）。
	if m["model"] != "glm-5.3[1m]" {
		t.Fatalf("model 应注入主模型: %v", m["model"])
	}
	// 全局未知字段保留。
	if _, ok := m["permissions"].(map[string]any); !ok {
		t.Fatalf("permissions 应保留: %v", m)
	}
}

// T008：providers.json 无 env（legacy）→ 零注入但构建成功（告警不阻断）。
func TestSyncProxy_NoEnvNoInjection(t *testing.T) {
	setTempRoot(t)
	home := setTempClaudeHome(t)
	os.MkdirAll(filepath.Join(home, "projects"), 0o700)

	dir, warnings, err := SyncProxy("bare", "http://127.0.0.1:48270")
	if err != nil {
		t.Fatalf("SyncProxy: %v", err)
	}
	m := map[string]any{}
	if err := json.Unmarshal([]byte(readProfileSettings(t, dir)), &m); err != nil {
		t.Fatal(err)
	}
	if _, exists := m["modelPicker"]; exists {
		t.Fatalf("无 env 不得注入 modelPicker: %v", m["modelPicker"])
	}
	if len(warnings) == 0 {
		t.Fatal("应给出 modelPlanUnavailable 告警")
	}
}
