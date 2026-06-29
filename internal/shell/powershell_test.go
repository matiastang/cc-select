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

func TestJoinChanges(t *testing.T) {
	if got := JoinChanges(nil); got != "" {
		t.Errorf("空切片应返回空串，got %q", got)
	}
	got := JoinChanges([]string{"a\n", "b\n", "c\n"})
	if got != "a\nb\nc\n" {
		t.Errorf("JoinChanges 拼接错误，got %q", got)
	}
}
