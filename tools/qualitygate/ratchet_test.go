package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseJscpdClones(t *testing.T) {
	t.Parallel()

	t.Run("标准 jscpd 报告", func(t *testing.T) {
		t.Parallel()
		input := `{"statistics":{"total":{"clones":10,"duplicatedLines":111,"percentage":1.05}}}`
		n, err := parseJscpdClones(strings.NewReader(input))
		if err != nil || n != 10 {
			t.Fatalf("clones=%d err=%v, want 10/nil", n, err)
		}
	})

	t.Run("缺 statistics 报错", func(t *testing.T) {
		t.Parallel()
		if _, err := parseJscpdClones(strings.NewReader(`{}`)); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestCountKnipIssues(t *testing.T) {
	t.Parallel()

	// knip --reporter json 的真实结构：{"issues":[...]}（对象包裹数组）。
	input := `{"issues":[
		{"file":"e2e/global-setup.ts","binaries":[{"name":"go"}],"exports":[],"types":[]},
		{"file":"src/a.ts","binaries":[],"exports":[{"name":"A"},{"name":"B"}],"types":[{"name":"T"}]}
	]}`
	n, err := countKnipIssues(strings.NewReader(input))
	if err != nil {
		t.Fatalf("countKnipIssues: %v", err)
	}
	if n != 4 { // 1 binary + 2 exports + 1 type
		t.Fatalf("issues=%d, want 4", n)
	}
}

func TestMetricCountDispatch(t *testing.T) {
	t.Parallel()

	t.Run("jscpd-* 前缀分发", func(t *testing.T) {
		t.Parallel()
		n, err := metricCount("jscpd-go", strings.NewReader(`{"statistics":{"total":{"clones":7}}}`))
		if err != nil || n != 7 {
			t.Fatalf("n=%d err=%v", n, err)
		}
	})

	t.Run("未知 metric 报错", func(t *testing.T) {
		t.Parallel()
		if _, err := metricCount("unknown", strings.NewReader(`{}`)); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestCompareCounter(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	if code := compareCounter("jscpd-go", 10, 10, &out); code != 0 {
		t.Fatalf("相等应通过: exit=%d", code)
	}
	if code := compareCounter("jscpd-go", 11, 10, &out); code != 1 {
		t.Fatalf("劣化应失败: exit=%d", code)
	}
	if code := compareCounter("jscpd-go", 8, 10, &out); code != 0 {
		t.Fatalf("改善应通过: exit=%d", code)
	}
	if !strings.Contains(out.String(), "基线可收紧到 8") {
		t.Fatalf("改善时应提示收紧基线，got:\n%s", out.String())
	}
}
