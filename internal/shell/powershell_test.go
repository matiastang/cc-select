package shell

import (
	"strings"
	"testing"
)

func TestPowerShellInitSnippet_ContainsBinaryPath(t *testing.T) {
	got := PowerShellEmitter{}.InitSnippet(`C:\tools\cc-select.exe`)
	if !strings.Contains(got, `C:\tools\cc-select.exe`) {
		t.Errorf("PS InitSnippet 应含二进制路径:\n%s", got)
	}
	if !strings.Contains(got, "ccs") {
		t.Errorf("PS InitSnippet 应定义 ccs 函数:\n%s", got)
	}
}

func TestPowerShellInitSnippet_UseJoinsOutputAsString(t *testing.T) {
	// ccs 的 use 分支用 Invoke-Expression（要求单个字符串参数）。
	// cc-select use 输出多行（$env:CLAUDE_CONFIG_DIR + $env:CC_SELECT_ACTIVE），
	// PowerShell 会按行拆成数组，直接传 Invoke-Expression 会报"无法将 Object[] 转 String"。
	// 故模板必须用 | Out-String 把输出合并成单字符串。
	got := PowerShellEmitter{}.InitSnippet(`C:\tools\cc-select.exe`)
	if !strings.Contains(got, "| Out-String") {
		t.Errorf("PS InitSnippet 的 use 分支应含 | Out-String（防 Invoke-Expression 数组报错）:\n%s", got)
	}
}

func TestJoinChanges(t *testing.T) {
	if got := JoinChanges(nil); got != "" {
		t.Errorf("空切片应返回空串，got %q", got)
	}
	got := JoinChanges([]string{"a\n", "b\n", "c\n"})
	if got != "a\nb\nc\n" {
		t.Errorf("JoinChanges 拼接错误，got %q", got)
	}
}
