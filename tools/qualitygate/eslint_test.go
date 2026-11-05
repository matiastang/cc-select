package main

import (
	"path"
	"strings"
	"testing"
)

func TestParseESLintJSON(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir() // 模拟 frontendDir 绝对路径
	input := `[
		{
			"filePath": "` + path.Join(baseDir, "src/features/providers/JsonForm.tsx") + `",
			"messages": [
				{"ruleId": "max-lines-per-function", "severity": 2, "line": 45, "message": "Function 'JsonForm' has too many lines (444). Maximum allowed is 150."},
				{"ruleId": "complexity", "severity": 2, "line": 45, "message": "Function 'JsonForm' has a complexity of 21. Maximum allowed is 15."},
				{"ruleId": "react-refresh/only-export-components", "severity": 1, "line": 258, "message": "Fast refresh only works..."}
			]
		},
		{
			"filePath": "` + path.Join(baseDir, "src/clean.ts") + `",
			"messages": []
		}
	]`

	findings, err := parseESLintJSON(strings.NewReader(input), baseDir)
	if err != nil {
		t.Fatalf("parseESLintJSON: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("findings = %d, want 2（warning severity=1 不计入）", len(findings))
	}
	first := findings[0]
	if first.File != "src/features/providers/JsonForm.tsx" {
		t.Fatalf("file 归一化失败: %q", first.File)
	}
	if first.Line != 45 || first.Rule != "max-lines-per-function" {
		t.Fatalf("first = %+v", first)
	}
}

func TestParseESLintJSON_EmptyAndGarbage(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()

	t.Run("空数组零 findings", func(t *testing.T) {
		t.Parallel()
		findings, err := parseESLintJSON(strings.NewReader("[]"), baseDir)
		if err != nil || len(findings) != 0 {
			t.Fatalf("err=%v findings=%d", err, len(findings))
		}
	})

	t.Run("非法 JSON 报错（配置错误语义）", func(t *testing.T) {
		t.Parallel()
		if _, err := parseESLintJSON(strings.NewReader("not json"), baseDir); err == nil {
			t.Fatal("want error")
		}
	})

	t.Run("路径逃出 baseDir 报错（绝不静默丢弃）", func(t *testing.T) {
		t.Parallel()
		input := `[{"filePath": "/somewhere/else/x.tsx", "messages": [{"ruleId": "max-lines", "severity": 2, "line": 1, "message": "m"}]}]`
		if _, err := parseESLintJSON(strings.NewReader(input), baseDir); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestStagedFrontendFiles(t *testing.T) {
	t.Parallel()

	t.Run("非前端 staged 文件被过滤", func(t *testing.T) {
		t.Parallel()
		got := frontendStagedFiles([]string{
			"internal/frontend/src/a.tsx",
			"internal/cli/add.go",
			"internal/frontend/package.json",
			"specs/005-quality-gates/tasks.md",
		})
		if len(got) != 1 || got[0] != "src/a.tsx" {
			t.Fatalf("got %v, want [src/a.tsx]", got)
		}
	})

	t.Run("空列表返回空", func(t *testing.T) {
		t.Parallel()
		if got := frontendStagedFiles(nil); len(got) != 0 {
			t.Fatalf("got %v, want empty", got)
		}
	})
}
