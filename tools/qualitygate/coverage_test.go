package main

import (
	"strings"
	"testing"
)

const sampleProfile = `mode: set
github.com/x/internal/a/a.go:10.20,12.3 2 1
github.com/x/internal/a/a.go:14.20,16.3 3 0
github.com/x/internal/b/b.go:5.12,9.2 5 1
`

func TestParseCoverProfile(t *testing.T) {
	t.Parallel()

	t.Run("总量与覆盖语句数", func(t *testing.T) {
		t.Parallel()
		got, err := parseCoverProfile(strings.NewReader(sampleProfile))
		if err != nil {
			t.Fatalf("parseCoverProfile: %v", err)
		}
		// 总语句 2+3+5=10，覆盖 2+5=7 → 70.0%
		if got.totalStmts != 10 || got.coveredStmts != 7 {
			t.Fatalf("total=%d covered=%d, want 10/7", got.totalStmts, got.coveredStmts)
		}
		if got.percent() < 69.99 || got.percent() > 70.01 {
			t.Fatalf("percent=%v, want 70.0", got.percent())
		}
	})

	t.Run("缺 mode 行报错", func(t *testing.T) {
		t.Parallel()
		if _, err := parseCoverProfile(strings.NewReader("a.go:1.1,2.2 1 1\n")); err == nil {
			t.Fatal("want error")
		}
	})

	t.Run("空 profile 报错（避免 0/0 假绿）", func(t *testing.T) {
		t.Parallel()
		if _, err := parseCoverProfile(strings.NewReader("mode: set\n")); err == nil {
			t.Fatal("want error")
		}
	})

	t.Run("非法行报错", func(t *testing.T) {
		t.Parallel()
		if _, err := parseCoverProfile(strings.NewReader("mode: set\ngarbage line\n")); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestRunCoverage(t *testing.T) {
	t.Parallel()

	baselineJSON := `{"version":1,"findings":{},"counters":{},"coverage":{"go-statements-min":76},"fileLines":{"go-max":1000}}`

	t.Run("达到阈值通过", func(t *testing.T) {
		t.Parallel()
		profile := "mode: set\n" + strings.Repeat("a.go:1.1,2.2 10 1\n", 8) +
			strings.Repeat("a.go:3.1,4.2 10 0\n", 2) // 80% ≥ 76
		baseline, err := LoadBaselineReader(strings.NewReader(baselineJSON))
		if err != nil {
			t.Fatal(err)
		}
		code := runCoverageWithProfile([]string{"--metric", "go-statements-min"}, strings.NewReader(profile), baseline)
		if code != 0 {
			t.Fatalf("exit=%d, want 0", code)
		}
	})

	t.Run("跌破阈值失败", func(t *testing.T) {
		t.Parallel()
		profile := "mode: set\n" + strings.Repeat("a.go:1.1,2.2 10 1\n", 7) +
			strings.Repeat("a.go:3.1,4.2 10 0\n", 3) // 70% < 76
		baseline, _ := LoadBaselineReader(strings.NewReader(baselineJSON))
		code := runCoverageWithProfile([]string{"--metric", "go-statements-min"}, strings.NewReader(profile), baseline)
		if code != 1 {
			t.Fatalf("exit=%d, want 1", code)
		}
	})

	t.Run("基线缺 metric 报配置错误", func(t *testing.T) {
		t.Parallel()
		profile := "mode: set\na.go:1.1,2.2 10 1\n"
		baseline, _ := LoadBaselineReader(strings.NewReader(baselineJSON))
		code := runCoverageWithProfile([]string{"--metric", "frontend-statements-min"}, strings.NewReader(profile), baseline)
		if code != 2 {
			t.Fatalf("exit=%d, want 2", code)
		}
	})
}
