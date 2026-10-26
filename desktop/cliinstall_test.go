package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// T013（specs/003-desktop-client/tasks.md）：内置 CLI 安装器四态。
// 契约 contracts/desktop-api.md §3.3：installed / reinstalled / upgraded /
// external-binary 409（confirmOverwrite 解锁）/ permission 409（~/.bin 回落）。
// 测试策略：TempDir + 真实可执行脚本（探测/替换经真实 exec），目标目录注入。

// writeFakeCLI 在 dir 写一个可执行的假 cc-select，--version 输出 ver。
func writeFakeCLI(t *testing.T, dir, ver string) string {
	t.Helper()
	_ = os.MkdirAll(dir, 0o755)
	p := filepath.Join(dir, "cc-select")
	script := "#!/bin/sh\n" + `echo "cc-select v` + ver + `"` + "\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// embedScript 返回一个"内置 CLI"写入函数：装到 dest 后 --version 输出 ver。
func embedScript(ver string) func(dest string) error {
	return func(dest string) error {
		script := "#!/bin/sh\n" + `echo "cc-select v` + ver + `"` + "\n"
		return os.WriteFile(dest, []byte(script), 0o755)
	}
}

func withLookPath(t *testing.T, dir string) {
	t.Helper()
	old := lookPathFn
	lookPathFn = func(name string) (string, error) {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() { lookPathFn = old })
}

func TestCliInstall_CleanInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix 脚本探测；windows 路径由 cliinstall_windows.go 覆盖")
	}
	target := t.TempDir()
	emptyPath := t.TempDir() // PATH 里没有 cc-select
	withLookPath(t, emptyPath)

	res, conflict, err := installEmbeddedCLI(installOptions{
		TargetDirs:       []string{target},
		EmbeddedVersion:  "0.1.0",
		ExtractEmbedded:  embedScript("0.1.0"),
		ConfirmOverwrite: false,
	})
	if err != nil || conflict != nil {
		t.Fatalf("clean install 应成功, conflict=%+v err=%v", conflict, err)
	}
	if res.Action != "installed" || res.InstalledVersion != "0.1.0" {
		t.Fatalf("res = %+v, want action=installed v0.1.0", res)
	}
	// 装完能执行且版本正确。
	out, err := exec.Command(res.InstalledPath, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != "cc-select v0.1.0\n" {
		t.Fatalf("installed --version = %q", got)
	}
}

func TestCliInstall_ReinstallSameVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix 脚本探测")
	}
	pathDir := t.TempDir()
	writeFakeCLI(t, pathDir, "0.1.0")
	withLookPath(t, pathDir)

	res, conflict, err := installEmbeddedCLI(installOptions{
		TargetDirs:      []string{pathDir},
		EmbeddedVersion: "0.1.0",
		ExtractEmbedded: embedScript("0.1.0"),
	})
	if err != nil || conflict != nil {
		t.Fatalf("same version reinstall 应幂等成功: %+v %v", conflict, err)
	}
	if res.Action != "reinstalled" {
		t.Fatalf("action = %s, want reinstalled", res.Action)
	}
}

func TestCliInstall_UpgradeOldVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix 脚本探测")
	}
	pathDir := t.TempDir()
	writeFakeCLI(t, pathDir, "0.0.7")
	withLookPath(t, pathDir)

	res, conflict, err := installEmbeddedCLI(installOptions{
		TargetDirs:      []string{pathDir},
		EmbeddedVersion: "0.1.0",
		ExtractEmbedded: embedScript("0.1.0"),
	})
	if err != nil || conflict != nil {
		t.Fatalf("升级应成功: %+v %v", conflict, err)
	}
	if res.Action != "upgraded" {
		t.Fatalf("action = %s, want upgraded", res.Action)
	}
}

func TestCliInstall_ExternalBinaryRequiresConfirm(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix 脚本探测")
	}
	pathDir := t.TempDir()
	// 外部同名二进制：--version 输出不是 cc-select 格式。
	p := filepath.Join(pathDir, "cc-select")
	_ = os.WriteFile(p, []byte("#!/bin/sh\necho 'some-other-tool 1.2'\n"), 0o755)
	withLookPath(t, pathDir)

	// 未确认 → external-binary 冲突。
	_, conflict, err := installEmbeddedCLI(installOptions{
		TargetDirs:      []string{pathDir},
		EmbeddedVersion: "0.1.0",
		ExtractEmbedded: embedScript("0.1.0"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if conflict == nil || conflict.Kind != conflictExternal {
		t.Fatalf("want external-binary conflict, got %+v", conflict)
	}

	// 确认后 → 覆盖。
	res, conflict, err := installEmbeddedCLI(installOptions{
		TargetDirs:       []string{pathDir},
		EmbeddedVersion:  "0.1.0",
		ExtractEmbedded:  embedScript("0.1.0"),
		ConfirmOverwrite: true,
	})
	if err != nil || conflict != nil {
		t.Fatalf("confirm 后应覆盖: %+v %v", conflict, err)
	}
	if res.Action != "overwritten" || res.InstalledPath != p {
		t.Fatalf("res = %+v, want overwritten at %s", res, p)
	}
}

func TestCliInstall_PermissionFallbackToWritableDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix 权限语义")
	}
	emptyPath := t.TempDir()
	withLookPath(t, emptyPath)

	// 只读目录 + 可写回落目录。
	readonly := t.TempDir()
	if err := os.Chmod(readonly, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readonly, 0o755) })
	fallback := t.TempDir()

	res, conflict, err := installEmbeddedCLI(installOptions{
		TargetDirs:      []string{readonly, fallback},
		EmbeddedVersion: "0.1.0",
		ExtractEmbedded: embedScript("0.1.0"),
	})
	if err != nil || conflict != nil {
		t.Fatalf("应回落到可写目录: %+v %v", conflict, err)
	}
	if res.InstalledPath != filepath.Join(fallback, "cc-select") {
		t.Fatalf("installedPath = %s, want fallback", res.InstalledPath)
	}
	if res.Hint == "" {
		t.Fatalf("回落安装必须带 hint（提示把目录加入 PATH）")
	}
}

func TestCliInstall_PermissionDeniedAllDirs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix 权限语义")
	}
	emptyPath := t.TempDir()
	withLookPath(t, emptyPath)

	readonly := t.TempDir()
	_ = os.Chmod(readonly, 0o500)
	t.Cleanup(func() { _ = os.Chmod(readonly, 0o755) })

	_, conflict, err := installEmbeddedCLI(installOptions{
		TargetDirs:      []string{readonly},
		EmbeddedVersion: "0.1.0",
		ExtractEmbedded: embedScript("0.1.0"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if conflict == nil || conflict.Kind != conflictPermission {
		t.Fatalf("want permission conflict, got %+v", conflict)
	}
	if conflict.Hint == "" {
		t.Fatalf("permission 冲突必须带平台指引 hint")
	}
}
