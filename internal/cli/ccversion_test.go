package cli

import (
	"testing"

	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/cc-select/cc-select/internal/profile"
)

// T009：claude --version 输出解析与 modelPicker 版本门槛（research D6）。
func TestSupportsModelPicker(t *testing.T) {
	cases := []struct {
		out  string
		want bool
	}{
		{"2.1.251 (Claude Code)", true},
		{"2.1.242 (Claude Code)", true},  // 门槛版本本身
		{"2.1.241 (Claude Code)", false}, // 门槛前一版
		{"2.1.220 (Claude Code)", false}, // 探针实验时的版本
		{"2.0.0", false},
		{"3.0.0 (Claude Code)", true}, // 未来主版本
		{"garbage output", false},     // 解析失败 → 不支持（保守）
		{"", false},
		{"claude version 2.1.250-extra", true}, // 前缀噪声容忍
	}
	for _, c := range cases {
		if got := supportsModelPicker(c.out); got != c.want {
			t.Errorf("supportsModelPicker(%q) = %v, want %v", c.out, got, c.want)
		}
	}
}

// T020 评审 finding 3：版本探测（子进程 claude --version，最坏 3s 超时）只应在
// 实际会注入选择器（plan 非空）时触发——无模型变量的 provider 不该让 use 热路径
// 白付一次子进程开销，提示文案也不会误导。
func TestUse_PickerUpgradeProbeGatedOnPlan(t *testing.T) {
	setTempCfg(t)
	writeProvidersWithModels(t) // glm/minimax 有模型变量；bare 无

	called := false
	orig := warnIfPickerUnsupported
	warnIfPickerUnsupported = func() string { called = true; return "" }
	defer func() { warnIfPickerUnsupported = orig }()

	// bare 无 profile 会拒绝 use——先建最小 profile。
	if _, _, err := profile.Sync("bare", map[string]string{"ANTHROPIC_BASE_URL": "https://bare.example.com"}, prefs.ModeSettingsOnly); err != nil {
		t.Fatalf("sync bare: %v", err)
	}

	// bare：providers.json 无 env 模型变量 → 不探测。
	if _, _, err := execRoot(t, "", "use", "bare"); err != nil {
		t.Fatalf("use bare: %v", err)
	}
	if called {
		t.Fatal("无模型变量的 provider 不应触发版本探测")
	}

	// glm：有模型变量 → 探测。
	if _, _, err := execRoot(t, "", "use", "glm"); err != nil {
		t.Fatalf("use glm: %v", err)
	}
	if !called {
		t.Fatal("有模型变量的 provider 应触发版本探测")
	}
}
