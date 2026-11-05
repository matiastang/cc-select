package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runFileLinesOK 执行 filelines 子命令并断言 exit 0。
func runFileLinesOK(t *testing.T, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runFileLines(args, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("filelines %v exit=%d, want 0\nstdout:\n%s\nstderr:\n%s",
			args, code, stdout.String(), stderr.String())
	}
	return stdout.String()
}

// writeGoFile 写入一个 n 行的 .go 文件。
func writeGoFile(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString("// padding line\n")
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestFileLines(t *testing.T) {
	t.Parallel()

	t.Run("超限文件检出且输出含路径与实测", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeGoFile(t, filepath.Join(root, "pkg/big.go"), 1200)

		out := runFileLinesCapture(t, []string{"--root", root, "--max", "1000"}, 1)
		if !strings.Contains(out, "pkg/big.go") || !strings.Contains(out, "1200") || !strings.Contains(out, "1000") {
			t.Fatalf("输出应含 file/实测/阈值，got:\n%s", out)
		}
	})

	t.Run("未超限通过且静默", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeGoFile(t, filepath.Join(root, "pkg/ok.go"), 900)
		runFileLinesOK(t, "--root", root, "--max", "1000")
	})

	t.Run("test 文件天然豁免", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeGoFile(t, filepath.Join(root, "pkg/big_test.go"), 2000)
		runFileLinesOK(t, "--root", root, "--max", "1000")
	})

	t.Run("排除目录跳过", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeGoFile(t, filepath.Join(root, "internal/frontend/node_modules/pkg/big.go"), 1200)
		writeGoFile(t, filepath.Join(root, "pkg/ok.go"), 10)
		runFileLinesOK(t, "--root", root, "--max", "1000",
			"--exclude", "internal/frontend/node_modules")
	})

	t.Run("递归子目录与多个超限文件全部列出", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeGoFile(t, filepath.Join(root, "a/a/one.go"), 1100)
		writeGoFile(t, filepath.Join(root, "b/two.go"), 1050)
		out := runFileLinesCapture(t, []string{"--root", root, "--max", "1000"}, 1)
		if !strings.Contains(out, "a/a/one.go") || !strings.Contains(out, "b/two.go") {
			t.Fatalf("应列出两个超限文件，got:\n%s", out)
		}
	})

	t.Run("无尾换行的最后一行计入", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		path := filepath.Join(root, "pkg", "noeol.go")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		var sb strings.Builder
		for i := 0; i < 1000; i++ {
			sb.WriteString("// x\n")
		}
		sb.WriteString("// last without eol") // 第 1001 行，无尾换行
		if err := os.WriteFile(path, []byte(sb.String()), 0o600); err != nil {
			t.Fatal(err)
		}
		out := runFileLinesCapture(t, []string{"--root", root, "--max", "1000"}, 1)
		if !strings.Contains(out, "1001") {
			t.Fatalf("应计为 1001 行，got:\n%s", out)
		}
	})

	t.Run("基线豁免的文件不报", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeGoFile(t, filepath.Join(root, "internal/web/api.go"), 1139)

		baseline := filepath.Join(t.TempDir(), "b.json")
		content := `{"version":1,"findings":{"filelines":[{"file":"internal/web/api.go","line":0,"rule":"file-lines"}]},"counters":{},"coverage":{},"fileLines":{"go-max":1000}}`
		if err := os.WriteFile(baseline, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		runFileLinesOK(t, "--root", root, "--max", "1000", "--baseline", baseline)
	})

	t.Run("基线未登记的文件仍报", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeGoFile(t, filepath.Join(root, "other/big.go"), 1200)

		baseline := filepath.Join(t.TempDir(), "b.json")
		content := `{"version":1,"findings":{"filelines":[{"file":"internal/web/api.go","line":0,"rule":"file-lines"}]},"counters":{},"coverage":{},"fileLines":{"go-max":1000}}`
		if err := os.WriteFile(baseline, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		runFileLinesCapture(t, []string{"--root", root, "--max", "1000", "--baseline", baseline}, 1)
	})

	t.Run("root 不存在报配置错误 exit=2", func(t *testing.T) {
		t.Parallel()
		runFileLinesCapture(t, []string{"--root", filepath.Join(t.TempDir(), "missing"), "--max", "1000"}, 2)
	})
}

// runFileLinesCapture 执行 filelines 并断言期望的退出码，返回 stdout。
func runFileLinesCapture(t *testing.T, args []string, wantCode int) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runFileLines(args, &stdout, &stderr)
	if code != wantCode {
		t.Fatalf("filelines %v exit=%d, want %d\nstdout:\n%s\nstderr:\n%s",
			args, code, wantCode, stdout.String(), stderr.String())
	}
	return stdout.String() + stderr.String()
}
