package rcinteg

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/shell"
)

// ---- composeManaged（纯函数）----

func TestComposeManaged_NoMarker_AppendsWithBlankLine(t *testing.T) {
	got := composeManaged("alias ll='ls -l'\n", "# block\n")
	want := "alias ll='ls -l'\n\n# block\n"
	if got != want {
		t.Errorf("append:\nwant %q\ngot  %q", want, got)
	}
}

func TestComposeManaged_NoMarker_EmptyOriginal(t *testing.T) {
	if got := composeManaged("", "# block\n"); got != "# block\n" {
		t.Errorf("空原文应只输出块: got %q", got)
	}
}

func TestComposeManaged_NoTrailingNewline(t *testing.T) {
	got := composeManaged("alias x=1", "# block\n")
	if !strings.HasPrefix(got, "alias x=1\n\n") {
		t.Errorf("应补换行+空行分隔: got %q", got)
	}
}

func TestComposeManaged_HasMarker_ReplacesBlock(t *testing.T) {
	original := "before\n" + markerBegin + "\nOLD\n" + markerEnd + "\nafter\n"
	// 真实 snippet 来自 RenderInit，自带 marker 块。
	snippet := markerBegin + "\nNEW\n" + markerEnd + "\n"
	got := composeManaged(original, snippet)
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Errorf("应保留块外内容: got %q", got)
	}
	if strings.Contains(got, "OLD") {
		t.Errorf("旧块内容应被替换: got %q", got)
	}
	if !strings.Contains(got, "NEW") {
		t.Errorf("应含新块: got %q", got)
	}
	if c := strings.Count(got, markerBegin); c != 1 {
		t.Errorf("begin 应恰有一个, got %d: %q", c, got)
	}
}

func TestComposeManaged_SameSnippet_IsNoop(t *testing.T) {
	snippet := markerBegin + "\nccs(){}\n" + markerEnd + "\n"
	if got := composeManaged(snippet, snippet); got != snippet {
		t.Errorf("相同 snippet 应原样返回(noop):\nwant %q\ngot  %q", snippet, got)
	}
}

func TestComposeManaged_OnlyBeginCorrupt_Recovers(t *testing.T) {
	original := "before\n" + markerBegin + "\nGARBAGE without end\n"
	got := composeManaged(original, "# NEW\n")
	if strings.Contains(got, "GARBAGE") {
		t.Errorf("损坏的 begin 之后应被丢弃: got %q", got)
	}
	if !strings.Contains(got, "before") || !strings.Contains(got, "# NEW") {
		t.Errorf("应保留 begin 之前并接新块: got %q", got)
	}
}

// ---- writeManagedBlock（IO, tempdir）----

func TestWriteManagedBlock_AppendsAndBacksUp(t *testing.T) {
	dir := t.TempDir()
	rc := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(rc, []byte("alias ll='ls -l'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snippet := markerBegin + "\nccs(){}\n" + markerEnd + "\n"

	action, err := writeManagedBlock(rc, snippet)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if action != ActionAppended {
		t.Errorf("首次应 appended, got %s", action)
	}
	bak, err := os.ReadFile(rc + ".cc-select.bak")
	if err != nil {
		t.Fatalf("备份应存在: %v", err)
	}
	if string(bak) != "alias ll='ls -l'\n" {
		t.Errorf("备份内容应为原文: got %q", bak)
	}
	got, _ := os.ReadFile(rc)
	if !strings.Contains(string(got), "alias ll='ls -l'") || !strings.Contains(string(got), "ccs()") {
		t.Errorf("写入后应含原文与新块: got %q", got)
	}
}

func TestWriteManagedBlock_IdempotentNoop(t *testing.T) {
	dir := t.TempDir()
	rc := filepath.Join(dir, ".zshrc")
	snippet := markerBegin + "\nccs(){}\n" + markerEnd + "\n"
	if _, err := writeManagedBlock(rc, snippet); err != nil {
		t.Fatal(err)
	}
	action, err := writeManagedBlock(rc, snippet) // 相同 snippet
	if err != nil {
		t.Fatal(err)
	}
	if action != ActionNoop {
		t.Errorf("相同 snippet 应 noop, got %s", action)
	}
	got, _ := os.ReadFile(rc)
	if c := strings.Count(string(got), markerBegin); c != 1 {
		t.Errorf("应只一个块, got %d", c)
	}
}

func TestWriteManagedBlock_UpdatesChangedSnippet(t *testing.T) {
	dir := t.TempDir()
	rc := filepath.Join(dir, ".zshrc")
	old := markerBegin + "\nOLD_BODY\n" + markerEnd + "\n"
	if _, err := writeManagedBlock(rc, old); err != nil {
		t.Fatal(err)
	}
	action, err := writeManagedBlock(rc, markerBegin+"\nNEW_BODY\n"+markerEnd+"\n")
	if err != nil {
		t.Fatal(err)
	}
	if action != ActionUpdated {
		t.Errorf("内容变化应 updated, got %s", action)
	}
	got, _ := os.ReadFile(rc)
	if strings.Contains(string(got), "OLD_BODY") || !strings.Contains(string(got), "NEW_BODY") {
		t.Errorf("应替换为新内容: got %q", got)
	}
}

func TestWriteManagedBlock_BackupNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	rc := filepath.Join(dir, ".zshrc")
	os.WriteFile(rc, []byte("V1\n"), 0o644)
	snippet := markerBegin + "\nccs(){}\n" + markerEnd + "\n"
	if _, err := writeManagedBlock(rc, snippet); err != nil {
		t.Fatal(err)
	}
	// 用户改了 rc 中块外内容后重装：marker 已在 → updated/noop，不应重写备份。
	os.WriteFile(rc, []byte("V1_CHANGED\n"+snippet), 0o644)
	if _, err := writeManagedBlock(rc, snippet); err != nil {
		t.Fatal(err)
	}
	bak, _ := os.ReadFile(rc + ".cc-select.bak")
	if string(bak) != "V1\n" {
		t.Errorf("备份不应被覆盖: got %q want %q", bak, "V1\n")
	}
}

func TestWriteManagedBlock_PreservesPermissions(t *testing.T) {
	dir := t.TempDir()
	rc := filepath.Join(dir, ".zshrc")
	os.WriteFile(rc, []byte("x\n"), 0o600)
	snippet := markerBegin + "\nccs(){}\n" + markerEnd + "\n"
	if _, err := writeManagedBlock(rc, snippet); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(rc)
	if err != nil {
		t.Fatal(err)
	}
	// Windows 忽略 Unix 权限位；只在 Unix 校验。
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("应保留原权限 0600, got %v", fi.Mode().Perm())
	}
}

// ---- RenderInit ----

func TestRenderInit_ZshContainsMarker(t *testing.T) {
	snippet, s, err := RenderInit("zsh")
	if err != nil {
		t.Fatal(err)
	}
	if s != shell.Zsh {
		t.Errorf("want zsh got %s", s)
	}
	if !strings.Contains(snippet, markerBegin) || !strings.Contains(snippet, markerEnd) {
		t.Errorf("snippet 应含 marker 块: %q", snippet)
	}
	if !strings.Contains(snippet, "ccs()") {
		t.Errorf("snippet 应含 ccs() 定义: %q", snippet)
	}
}

func TestRenderInit_PowerShellSnippet(t *testing.T) {
	snippet, s, err := RenderInit("powershell")
	if err != nil {
		t.Fatal(err)
	}
	if s != shell.PowerShell {
		t.Errorf("want powershell got %s", s)
	}
	if !strings.Contains(snippet, "function ccs") || !strings.Contains(snippet, markerBegin) {
		t.Errorf("ps snippet 应含 function ccs + marker: %q", snippet)
	}
}

func TestRenderInit_UnsupportedShellErrors(t *testing.T) {
	if _, _, err := RenderInit("fish"); err == nil {
		t.Error("fish 应在 RenderInit 失败（shell.For 不支持）")
	}
}

// ---- Resolve（策略）----

func TestResolve_ZshAndBash(t *testing.T) {
	home := t.TempDir()
	if got, err := (unixStrategy{shell: shell.Zsh}).Resolve(home); err != nil || got != filepath.Join(home, ".zshrc") {
		t.Errorf("zsh: got %s err %v", got, err)
	}
	// 都无 → .bashrc
	if got, err := (unixStrategy{shell: shell.Bash}).Resolve(home); err != nil || got != filepath.Join(home, ".bashrc") {
		t.Errorf("bash 默认 .bashrc: got %s err %v", got, err)
	}
	// 有 .bash_profile 无 .bashrc → .bash_profile
	profile := filepath.Join(home, ".bash_profile")
	os.WriteFile(profile, []byte("# login\n"), 0o644)
	if got, err := (unixStrategy{shell: shell.Bash}).Resolve(home); err != nil || got != profile {
		t.Errorf("有 .bash_profile 应选它: got %s err %v", got, err)
	}
}

// ---- installWith manual 降级（注入 mock strategy）----

type failStrategy struct{}

func (failStrategy) Resolve(string) (string, error) { return "", errors.New("boom") }

func TestInstallWith_ManualWhenResolveFails(t *testing.T) {
	res, err := installWith(shell.PowerShell, "SNIPPET", failStrategy{})
	if err != nil {
		t.Fatalf("不应 error（降级为 manual）: %v", err)
	}
	if res.Action != ActionManual || res.Snippet != "SNIPPET" {
		t.Errorf("应 manual+返回 snippet: %+v", res)
	}
}

// ---- DetectStatus / Install 端到端（unix shell, home 注入）----

func setHomeEnv(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
	} else {
		t.Setenv("HOME", dir)
	}
}

func TestDetectStatus_FreshZsh(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CC_SELECT_SHELL", "zsh")
	setHomeEnv(t, home)

	st := DetectStatus()
	if !st.Supported || st.Installed {
		t.Errorf("全新环境应 supported+!installed: %+v", st)
	}
	if st.RCPath != filepath.Join(home, ".zshrc") {
		t.Errorf("rcPath: got %s", st.RCPath)
	}
	if !st.CanAutoInstall {
		t.Error("zsh 应可自动安装")
	}
}

func TestInstall_Zsh_AppendsThenNoop(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CC_SELECT_SHELL", "zsh")
	setHomeEnv(t, home)

	r1, err := Install("zsh")
	if err != nil {
		t.Fatal(err)
	}
	if r1.Action != ActionAppended {
		t.Errorf("首次应 appended, got %s", r1.Action)
	}
	if st := DetectStatus(); !st.Installed {
		t.Error("安装后 DetectStatus 应报 installed")
	}
	if r2, _ := Install("zsh"); r2.Action != ActionNoop {
		t.Errorf("二次相同应 noop, got %s", r2.Action)
	}
}

func TestDetectStatus_DetectsLegacy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CC_SELECT_SHELL", "zsh")
	setHomeEnv(t, home)
	// 老的无 marker ccs() 块。
	os.WriteFile(filepath.Join(home, ".zshrc"), []byte("ccs() { echo old; }\n"), 0o644)

	st := DetectStatus()
	if st.Installed {
		t.Error("无 marker 不应算 installed")
	}
	if !st.Legacy {
		t.Error("应检测到 legacy ccs()")
	}
}

// ---- review 修复后的回归 ----

func TestRenderInit_NormalizesCase(t *testing.T) {
	for _, name := range []string{"PowerShell", "ZSH", "BaSh"} {
		_, s, err := RenderInit(name)
		if err != nil {
			t.Errorf("RenderInit(%q) 应接受自然大小写, got err %v", name, err)
		}
		if s == shell.Unknown {
			t.Errorf("RenderInit(%q) 不应解析为 Unknown", name)
		}
	}
}

func TestComposeManaged_RemovesAllButOneBlock(t *testing.T) {
	blk := func(body string) string { return markerBegin + "\n" + body + "\n" + markerEnd + "\n" }
	original := "head\n" + blk("A") + "mid\n" + blk("B") + "tail\n"
	got := composeManaged(original, blk("NEW"))
	if c := strings.Count(got, markerBegin); c != 1 {
		t.Errorf("应只剩一个块, got %d: %q", c, got)
	}
	if strings.Contains(got, "\nA\n") || strings.Contains(got, "\nB\n") {
		t.Errorf("旧块体应被移除: %q", got)
	}
	for _, want := range []string{"head", "mid", "tail", "NEW"} {
		if !strings.Contains(got, want) {
			t.Errorf("应保留 %q 并写入新块: %q", want, got)
		}
	}
}

func TestComposeManaged_MidSentenceMarkerSubstringPreserved(t *testing.T) {
	// 用户注释「句中」含 marker 子串不应被当作块开始（行前缀匹配，避免误删）。
	original := "# see # >>> cc-select shell integration docs for info\nalias x=1\n"
	got := composeManaged(original, markerBegin+"\nNEW\n"+markerEnd+"\n")
	if !strings.Contains(got, "alias x=1") || !strings.Contains(got, "docs for info") {
		t.Errorf("句中子串不应触发块移除/截断: %q", got)
	}
}

func TestWriteManagedBlock_BackupCreatedOnUpdate(t *testing.T) {
	dir := t.TempDir()
	rc := filepath.Join(dir, ".zshrc")
	// 预置：已有 marker 块但无备份（marker 存在却缺备份的边缘情况）。
	old := markerBegin + "\nOLD_BODY\n" + markerEnd + "\n"
	if err := os.WriteFile(rc, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := writeManagedBlock(rc, markerBegin+"\nNEW_BODY\n"+markerEnd+"\n"); err != nil {
		t.Fatal(err)
	}
	bak, err := os.ReadFile(rc + ".cc-select.bak")
	if err != nil {
		t.Fatalf("update 路径也应产生备份: %v", err)
	}
	if !strings.Contains(string(bak), "OLD_BODY") {
		t.Errorf("备份应含更新前的内容: %s", bak)
	}
}

func TestWriteManagedBlock_SymlinkPreserved(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("符号链接需特权，Windows 跳过")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "real-zshrc")
	if err := os.WriteFile(target, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, ".zshrc")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("无法创建符号链接: " + err.Error())
	}
	snippet := markerBegin + "\nccs(){}\n" + markerEnd + "\n"
	if _, err := writeManagedBlock(link, snippet); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("软链被替换成普通文件（dotfiles 用户配置会被破坏）")
	}
	got, _ := os.ReadFile(target)
	if !strings.Contains(string(got), "original") || !strings.Contains(string(got), "ccs()") {
		t.Errorf("应写入软链指向的真实文件: %s", got)
	}
}

func TestResolve_BashPrefersProfileWhenBothExist(t *testing.T) {
	home := t.TempDir()
	os.WriteFile(filepath.Join(home, ".bash_profile"), []byte("# login\n"), 0o644)
	os.WriteFile(filepath.Join(home, ".bashrc"), []byte("# interactive\n"), 0o644)
	got, err := (unixStrategy{shell: shell.Bash}).Resolve(home)
	if err != nil || got != filepath.Join(home, ".bash_profile") {
		t.Errorf("两者都在时应选 .bash_profile（登录 bash 读它）: got %s err %v", got, err)
	}
}

func TestInstall_UnsupportedShellReturnsManual(t *testing.T) {
	// fish：shell.For 不支持。Install 应返回 manual（非 error），让 API 给结构化响应而非 500。
	res, err := Install("fish")
	if err != nil {
		t.Fatalf("不支持 shell 不应 error: %v", err)
	}
	if res.Action != ActionManual {
		t.Errorf("应 manual, got %s", res.Action)
	}
}

func TestResolveShell_NormalizesAndFallsBack(t *testing.T) {
	t.Setenv("CC_SELECT_SHELL", "bash")
	if got := resolveShell(""); got != shell.Bash {
		t.Errorf("空串应回退 Detect, got %s", got)
	}
	if got := resolveShell("  PowerShell "); got != shell.PowerShell {
		t.Errorf("应归一化空白+大小写, got %s", got)
	}
}

func TestHasLegacy_PerShell(t *testing.T) {
	if !hasLegacy("function ccs { echo old }", shell.PowerShell) {
		t.Error("应检测 PS legacy function ccs")
	}
	if hasLegacy("alias ccs=echo", shell.PowerShell) {
		t.Error("非函数不应误报 PS legacy")
	}
	if !hasLegacy("ccs() { echo old; }", shell.Zsh) {
		t.Error("应检测 zsh legacy ccs()")
	}
}

// ---- PowerShell 探测（注入 profileProbe，不依赖真实 pwsh）----

// withProbe 临时替换 profileProbe 并重置缓存，测试结束还原。
func withProbe(t *testing.T, probe func() (string, error)) {
	t.Helper()
	resetPwshCache()
	old := profileProbe
	profileProbe = probe
	t.Cleanup(func() {
		profileProbe = old
		resetPwshCache()
	})
}

func TestPwshResolve_CachesResult(t *testing.T) {
	calls := 0
	withProbe(t, func() (string, error) {
		calls++
		return "/x/profile.ps1", nil
	})
	p := pwshStrategy{}
	got1, err1 := p.Resolve("")
	got2, err2 := p.Resolve("")
	if err1 != nil || err2 != nil {
		t.Fatalf("应无错: %v %v", err1, err2)
	}
	if got1 != "/x/profile.ps1" || got1 != got2 {
		t.Errorf("应返回探测路径且一致: %s %s", got1, got2)
	}
	if calls != 1 {
		t.Errorf("应缓存只探测一次, got %d", calls)
	}
}

func TestPwshResolve_PropagatesError(t *testing.T) {
	withProbe(t, func() (string, error) { return "", errors.New("boom") })
	p := pwshStrategy{}
	if _, err := p.Resolve(""); err == nil {
		t.Error("应返回探测错误")
	}
}

func TestDetectStatus_PwshUnresolvableIsManual(t *testing.T) {
	withProbe(t, func() (string, error) { return "", errors.New("no pwsh") })
	t.Setenv("CC_SELECT_SHELL", "powershell")
	st := DetectStatus()
	if !st.Supported {
		t.Error("powershell 应 supported")
	}
	if st.CanAutoInstall {
		t.Error("pwsh 探测失败应 CanAutoInstall=false（前端走手动降级）")
	}
}

func TestInstall_PwshManualReturnsSnippet(t *testing.T) {
	withProbe(t, func() (string, error) { return "", errors.New("no pwsh") })
	t.Setenv("CC_SELECT_SHELL", "powershell")
	res, err := Install("powershell")
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionManual {
		t.Errorf("pwsh 不可写应 manual, got %s", res.Action)
	}
	if !strings.Contains(res.Snippet, "function ccs") {
		t.Errorf("manual 应附 PS snippet 供用户粘贴: %q", res.Snippet)
	}
}
