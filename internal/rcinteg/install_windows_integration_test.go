//go:build windows && integration

package rcinteg

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestInstall_PowerShellProfileIsLoadable 是发版前的 Windows 安全网:真实跑一遍
// PowerShell 升级链路(Install → 写 $PROFILE),断言写出的 .ps1 带 UTF-8 BOM,且
// Windows PowerShell 5.1(powershell.exe)与 PowerShell 7(pwsh)都能加载它并定义 ccs。
//
// 防回归:本次「PS 5.1 对无 BOM 的 .ps1 按 GBK 读取,UTF-8 中文注释字节错位破坏语法」
// 的坑——单测 atomicWriteRC 只验字节,本测试验「PowerShell 真能加载」这一最终事实。
//
// 在 GitHub Actions windows-latest runner 上跑(runner 默认不开 Smart App Control)。
func TestInstall_PowerShellProfileIsLoadable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home) // 隔离 home,Install 写出的 $PROFILE 落临时目录
	t.Setenv("CC_SELECT_SHELL", "powershell")
	resetPwshCache() // 让 Install 重新探测 $PROFILE(避免跨测试缓存污染)

	res, err := Install("powershell")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Action == ActionManual || res.RCPath == "" {
		t.Skipf("未装 PowerShell 或 $PROFILE 不可写,跳过: %s", res.Message)
	}
	t.Logf("写入 %s (action=%s)", res.RCPath, res.Action)

	// ① 写出的 .ps1 必须带 UTF-8 BOM(PS 5.1 靠它识别 UTF-8)。
	data, err := os.ReadFile(res.RCPath)
	if err != nil {
		t.Fatalf("读取 $PROFILE: %v", err)
	}
	if !bytes.HasPrefix(data, utf8BOM) {
		t.Fatalf("$PROFILE 缺 UTF-8 BOM——PS 5.1 会按 GBK 读,中文注释将破坏语法")
	}
	if !strings.Contains(string(data), "function ccs") {
		t.Errorf("$PROFILE 应含 ccs 函数定义")
	}

	// ② 给定 PS 解释器,dot-source 该 profile 后 ccs 应被定义为 Function。
	loadCheck := func(t *testing.T, exe, label string) {
		t.Helper()
		out, err := exec.Command(exe, "-NoProfile", "-Command",
			". '"+res.RCPath+"'; (Get-Command ccs -ErrorAction SilentlyContinue).CommandType",
		).CombinedOutput()
		if err != nil {
			t.Fatalf("%s 加载 $PROFILE 失败: %v\n%s", label, err, out)
		}
		if !strings.Contains(string(out), "Function") {
			t.Errorf("%s 加载后 ccs 未定义为 Function:\n%s", label, out)
		}
		t.Logf("%s 加载 OK", label)
	}

	// Windows PowerShell 5.1(本次坑的受害者)必须通过。
	loadCheck(t, "powershell.exe", "PS 5.1")

	// 若装了 PowerShell 7(pwsh),同样验证(它默认 UTF-8,本应无碍)。
	if _, err := exec.LookPath("pwsh"); err == nil {
		loadCheck(t, "pwsh", "PS 7")
	}
}
