package cli

import "testing"

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
