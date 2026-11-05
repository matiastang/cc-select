package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBaseline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantErr bool
		check   func(t *testing.T, b *Baseline)
	}{
		{
			name: "合法基线完整解析",
			content: `{
				"version": 1,
				"findings": {
					"golangci": [
						{"file": "internal/cli/add.go", "line": 211, "rule": "cyclop", "message": "calculated cyclomatic complexity for function readProviderInput"}
					]
				},
				"counters": {"jscpd-go": {"count": 10, "metric": "clones"}},
				"coverage": {"go-statements-min": 76},
				"fileLines": {"go-max": 1000}
			}`,
			check: func(t *testing.T, b *Baseline) {
				if b.Version != 1 {
					t.Fatalf("version = %d, want 1", b.Version)
				}
				if got := len(b.Findings["golangci"]); got != 1 {
					t.Fatalf("golangci findings = %d, want 1", got)
				}
				if b.Counters["jscpd-go"].Count != 10 {
					t.Fatalf("jscpd-go count = %d, want 10", b.Counters["jscpd-go"].Count)
				}
				if b.Coverage["go-statements-min"] != 76 {
					t.Fatalf("go-statements-min = %v, want 76", b.Coverage["go-statements-min"])
				}
				if b.FileLines.GoMax != 1000 {
					t.Fatalf("go-max = %d, want 1000", b.FileLines.GoMax)
				}
			},
		},
		{
			name:    "version 缺失报错",
			content: `{"findings": {}, "counters": {}, "coverage": {}, "fileLines": {"go-max": 1000}}`,
			wantErr: true,
		},
		{
			name:    "version 不是 1 报错（格式演进入口）",
			content: `{"version": 2, "findings": {}, "counters": {}, "coverage": {}, "fileLines": {"go-max": 1000}}`,
			wantErr: true,
		},
		{
			name:    "非法 JSON 报错",
			content: `{not json}`,
			wantErr: true,
		},
		{
			name:    "finding 缺 file 报错",
			content: `{"version": 1, "findings": {"golangci": [{"line": 1, "rule": "cyclop"}]}, "counters": {}, "coverage": {}, "fileLines": {"go-max": 1}}`,
			wantErr: true,
		},
		{
			name:    "finding 缺 rule 报错",
			content: `{"version": 1, "findings": {"golangci": [{"file": "a.go", "line": 1}]}, "counters": {}, "coverage": {}, "fileLines": {"go-max": 1}}`,
			wantErr: true,
		},
		{
			name:    "counter 负数报错",
			content: `{"version": 1, "findings": {}, "counters": {"x": {"count": -1}}, "coverage": {}, "fileLines": {"go-max": 1}}`,
			wantErr: true,
		},
		{
			name:    "coverage 越界报错",
			content: `{"version": 1, "findings": {}, "counters": {}, "coverage": {"go-min": 101}, "fileLines": {"go-max": 1}}`,
			wantErr: true,
		},
		{
			name:    "文件不存在报错",
			content: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "quality-baseline.json")
			if tt.content != "" {
				if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			b, err := LoadBaseline(path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("LoadBaseline(%q) error = nil, want error", path)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadBaseline(%q) error = %v, want nil", path, err)
			}
			if tt.check != nil {
				tt.check(t, b)
			}
		})
	}
}

func TestFindingMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		baseline  Finding
		candidate Finding
		want      bool
	}{
		{
			name:      "file+line+rule 全匹配",
			baseline:  Finding{File: "internal/cli/add.go", Line: 211, Rule: "cyclop"},
			candidate: Finding{File: "internal/cli/add.go", Line: 211, Rule: "cyclop"},
			want:      true,
		},
		{
			name:      "line 不同不匹配",
			baseline:  Finding{File: "a.go", Line: 10, Rule: "cyclop"},
			candidate: Finding{File: "a.go", Line: 11, Rule: "cyclop"},
			want:      false,
		},
		{
			name:      "rule 不同不匹配",
			baseline:  Finding{File: "a.go", Line: 10, Rule: "cyclop"},
			candidate: Finding{File: "a.go", Line: 10, Rule: "funlen"},
			want:      false,
		},
		{
			name:      "file 不同不匹配",
			baseline:  Finding{File: "a.go", Line: 10, Rule: "cyclop"},
			candidate: Finding{File: "b.go", Line: 10, Rule: "cyclop"},
			want:      false,
		},
		{
			name:      "基线 line=0 整文件级匹配任意行",
			baseline:  Finding{File: "a.go", Line: 0, Rule: "cyclop"},
			candidate: Finding{File: "a.go", Line: 999, Rule: "cyclop"},
			want:      true,
		},
		{
			name:      "message 前缀匹配（容忍数值变化）",
			baseline:  Finding{File: "a.go", Line: 10, Rule: "cyclop", Message: "calculated cyclomatic complexity for function foo"},
			candidate: Finding{File: "a.go", Line: 10, Rule: "cyclop", Message: "calculated cyclomatic complexity for function foo is 26, max is 15"},
			want:      true,
		},
		{
			name:      "message 不满足前缀不匹配",
			baseline:  Finding{File: "a.go", Line: 10, Rule: "cyclop", Message: "calculated cyclomatic complexity for function foo"},
			candidate: Finding{File: "a.go", Line: 10, Rule: "cyclop", Message: "calculated cyclomatic complexity for function bar is 26, max is 15"},
			want:      false,
		},
		{
			name:      "基线 message 为空则忽略 message",
			baseline:  Finding{File: "a.go", Line: 10, Rule: "cyclop"},
			candidate: Finding{File: "a.go", Line: 10, Rule: "cyclop", Message: "anything"},
			want:      true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.baseline.matches(tt.candidate); got != tt.want {
				t.Fatalf("matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBaselineHasFinding(t *testing.T) {
	t.Parallel()

	b := &Baseline{
		Version: 1,
		Findings: map[string][]Finding{
			"golangci": {
				{File: "internal/cli/add.go", Line: 211, Rule: "cyclop"},
			},
			"eslint": {
				{File: "src/features/providers/JsonForm.tsx", Line: 0, Rule: "max-lines"},
			},
		},
	}

	if !b.HasFinding("golangci", Finding{File: "internal/cli/add.go", Line: 211, Rule: "cyclop"}) {
		t.Fatal("golangci 基线条目应命中")
	}
	if b.HasFinding("golangci", Finding{File: "internal/usage/usage_test.go", Line: 14, Rule: "unused"}) {
		t.Fatal("未登记条目不应命中")
	}
	if !b.HasFinding("eslint", Finding{File: "src/features/providers/JsonForm.tsx", Line: 1, Rule: "max-lines"}) {
		t.Fatal("eslint line=0 整文件条目应命中任意行")
	}
	// 未知 tool 组：空基线不应命中任何条目
	if b.HasFinding("knip", Finding{File: "x.ts", Line: 0, Rule: "unused"}) {
		t.Fatal("未知 tool 不应命中")
	}
}
