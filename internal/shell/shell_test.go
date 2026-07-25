package shell

import (
	"strings"
	"testing"
)

func TestZshEmit_SetUnset(t *testing.T) {
	got := ZshEmitter{}.Emit([]Change{
		{Op: OpSet, Name: "ANTHROPIC_MODEL", Value: "glm-4.6"},
		{Op: OpUnset, Name: "ANTHROPIC_BASE_URL"},
	})
	want := "export ANTHROPIC_MODEL='glm-4.6'\nunset ANTHROPIC_BASE_URL\n"
	if got != want {
		t.Errorf("Emit:\nwant %q\ngot  %q", want, got)
	}
}

func TestZshEmit_SpecialCharsEscaped(t *testing.T) {
	// 含单引号、$、空格、反引号的值必须被安全转义，不被 shell 解释。
	got := ZshEmitter{}.Emit([]Change{
		{Op: OpSet, Name: "K", Value: "a'b$c `d e"},
	})
	if !strings.Contains(got, "'a'\\''b$c `d e'") {
		t.Errorf("单引号转义失败: %q", got)
	}
	if strings.Contains(got, "$K") { // 误把值里的 $ 当变量
		t.Errorf("值中 $ 应在引号内不被解释: %q", got)
	}
}

func TestZshEmit_EmptyValue(t *testing.T) {
	got := ZshEmitter{}.Emit([]Change{{Op: OpSet, Name: "X", Value: ""}})
	if got != "export X=''\n" {
		t.Errorf("空值: want export X='' got %q", got)
	}
}

func TestZshInitSnippet_ContainsBinaryPath(t *testing.T) {
	got := ZshEmitter{}.InitSnippet("/usr/local/bin/cc-select")
	if !strings.Contains(got, "/usr/local/bin/cc-select") {
		t.Errorf("InitSnippet 应含二进制路径: %q", got)
	}
	if !strings.Contains(got, "eval") {
		t.Errorf("InitSnippet 应含 eval（use 走 eval）: %q", got)
	}
	if !strings.Contains(got, "ccs()") {
		t.Errorf("InitSnippet 应定义 ccs 函数: %q", got)
	}
}

func TestPowerShellEmit(t *testing.T) {
	got := PowerShellEmitter{}.Emit([]Change{
		{Op: OpSet, Name: "X", Value: "a'b"},
		{Op: OpUnset, Name: "Y"},
	})
	if !strings.Contains(got, "$env:X = 'a''b'") {
		t.Errorf("PS set/转义失败: %q", got)
	}
	if !strings.Contains(got, "Remove-Item Env:\\Y") {
		t.Errorf("PS unset 失败: %q", got)
	}
}

func TestFor_Dispatch(t *testing.T) {
	if _, err := For(Zsh); err != nil {
		t.Errorf("Zsh 应可用: %v", err)
	}
	if _, err := For(Bash); err != nil { // bash 复用 ZshEmitter
		t.Errorf("Bash 应可用: %v", err)
	}
	if _, err := For(PowerShell); err != nil {
		t.Errorf("PowerShell 应可用: %v", err)
	}
	if _, err := For(Unknown); err == nil {
		t.Error("Unknown shell 应返回错误")
	}
}

func TestDetect_Override(t *testing.T) {
	t.Setenv("CC_SELECT_SHELL", "powershell")
	if got := Detect(); got != PowerShell {
		t.Errorf("CC_SELECT_SHELL 覆盖: want powershell got %s", got)
	}
}

func TestDetect_DefaultZshOnUnix(t *testing.T) {
	t.Setenv("CC_SELECT_SHELL", "")
	t.Setenv("SHELL", "")
	// Unix 上期望 Zsh，Windows 上期望 PowerShell；这里只验证返回已知值、不 panic。
	got := Detect()
	if got == Unknown {
		t.Errorf("Detect 不应返回 Unknown（应有平台默认）")
	}
}

// TestInitSnippets_AreASCIIOnly 锁定：cc-select init 生成的 ccs() 代码必须纯 ASCII。
// 这些输出会被 shell 重定向捕获（>> $PROFILE / >> ~/.zshrc）。Windows PowerShell 5.1
// 按控制台代码页（中文系统=GBK）解码外部命令 stdout，非 ASCII（如中文）注释会被错位
// 解码、吞掉换行，破坏生成的 rc 语法。防止日后"贴心"地重新本地化注释而引入回归。
func TestInitSnippets_AreASCIIOnly(t *testing.T) {
	snippets := map[string]string{
		"zsh":        ZshEmitter{}.InitSnippet("/usr/local/bin/cc-select"),
		"powershell": PowerShellEmitter{}.InitSnippet(`C:\tools\cc-select.exe`),
	}
	for name, snip := range snippets {
		for _, r := range snip {
			if r > 127 {
				t.Errorf("%s InitSnippet 必须纯 ASCII，发现非 ASCII 字符 %q:\n%s", name, r, snip)
			}
		}
	}
}
