package updater

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// setupRun 准备一次 Run 的公共依赖：版本号、安装上下文（temp stub exe）、锁目录。
// 返回 target 路径。
func setupRun(t *testing.T, curVersion string) string {
	t.Helper()
	withVersion(t, curVersion)
	dir := t.TempDir()
	target := filepath.Join(dir, binNameForOS())
	if err := os.WriteFile(target, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	withDetectContext(t, Context{
		ExecPath: target,
		IsDev:    false,
		Manager:  ManagerNone,
		Writable: true,
	})
	withLockDir(t, t.TempDir())
	return target
}

func TestRun_DevBuildRefused(t *testing.T) {
	withVersion(t, "dev")
	withLockDir(t, t.TempDir())
	withDetectContext(t, Context{ExecPath: "/x/cc-select", IsDev: true, Writable: true})
	rel := &fakeReleaser{}

	_, err := Run(context.Background(), Options{Releaser: rel})
	var refused *RefusedError
	if !errors.As(err, &refused) || refused.Kind != KindDev {
		t.Fatalf("应拒绝 dev, got %v", err)
	}
	if rel.calls != 0 {
		t.Error("拒绝后不应访问网络")
	}
}

func TestRun_HomebrewRefused(t *testing.T) {
	withDetectContext(t, Context{
		ExecPath: "/opt/homebrew/Cellar/cc-select/1.0.0/bin/cc-select",
		IsDev:    false,
		Manager:  ManagerHomebrew,
		Writable: true,
	})
	withLockDir(t, t.TempDir())
	rel := &fakeReleaser{}

	_, err := Run(context.Background(), Options{Releaser: rel})
	var refused *RefusedError
	if !errors.As(err, &refused) || refused.Kind != KindHomebrew {
		t.Fatalf("应拒绝 homebrew, got %v", err)
	}
	if rel.calls != 0 {
		t.Error("拒绝后不应访问网络")
	}
}

func TestRun_ScoopRefused(t *testing.T) {
	withDetectContext(t, Context{
		ExecPath: `C:\Users\x\scoop\apps\cc-select\current\cc-select.exe`,
		IsDev:    false,
		Manager:  ManagerScoop,
		Writable: true,
	})
	withLockDir(t, t.TempDir())

	_, err := Run(context.Background(), Options{Releaser: &fakeReleaser{}})
	var refused *RefusedError
	if !errors.As(err, &refused) || refused.Kind != KindScoop {
		t.Fatalf("应拒绝 scoop, got %v", err)
	}
}

func TestRun_NotWritableRefused(t *testing.T) {
	withDetectContext(t, Context{ExecPath: "/usr/local/bin/cc-select", IsDev: false, Writable: false})
	withLockDir(t, t.TempDir())

	_, err := Run(context.Background(), Options{Releaser: &fakeReleaser{}})
	var refused *RefusedError
	if !errors.As(err, &refused) || refused.Kind != KindNotWritable {
		t.Fatalf("应拒绝不可写, got %v", err)
	}
}

func TestRun_UpToDateNoInstall(t *testing.T) {
	setupRun(t, "1.2.0")
	rel, _, _ := makeRelease(t, "1.2.0", "payload")
	replacer := &recordingReplacer{}

	out, err := Run(context.Background(), Options{
		Releaser: &fakeReleaser{rel: rel},
		Replacer: replacer,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Installed {
		t.Error("已最新不应安装")
	}
	for _, c := range replacer.calls {
		if c == "Replace" {
			t.Error("已最新不应调用 Replace")
		}
	}
}

func TestRun_HappyPath(t *testing.T) {
	target := setupRun(t, "1.0.0")
	payload := "new-binary-payload-" + runtime.GOOS
	rel, archive, checksums := makeRelease(t, "1.2.0", payload)
	replacer := &recordingReplacer{}

	out, err := Run(context.Background(), Options{
		Releaser: &fakeReleaser{rel: rel},
		Fetcher:  fetcherFor(rel, map[string][]byte{rel.Assets[0].Name: archive, checksumsAssetName: checksums}),
		Replacer: replacer,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !out.Installed || out.ToVersion != "1.2.0" || out.FromVersion != "1.0.0" {
		t.Errorf("Outcome 错误: %+v", out)
	}
	got, _ := os.ReadFile(target)
	if string(got) != payload {
		t.Errorf("target 内容 = %q, want %q", got, payload)
	}
	// 调用序列：CleanupStaleBackup 在 Replace 之前。
	if len(replacer.calls) != 2 || replacer.calls[0] != "CleanupStaleBackup" || replacer.calls[1] != "Replace" {
		t.Errorf("调用序列 = %v, want [CleanupStaleBackup Replace]", replacer.calls)
	}
}

func TestRun_ChecksumMismatch_OldBinaryUntouched(t *testing.T) {
	target := setupRun(t, "1.0.0")
	rel, archive, _ := makeRelease(t, "1.2.0", "payload")
	badChecksums := []byte(fmt.Sprintf("%s  %s\n", "deadbeef", rel.Assets[0].Name))
	replacer := &recordingReplacer{}

	_, err := Run(context.Background(), Options{
		Releaser: &fakeReleaser{rel: rel},
		Fetcher:  fetcherFor(rel, map[string][]byte{rel.Assets[0].Name: archive, checksumsAssetName: badChecksums}),
		Replacer: replacer,
	})
	if err == nil {
		t.Fatal("checksum 不匹配应报错")
	}
	for _, c := range replacer.calls {
		if c == "Replace" {
			t.Error("checksum 不匹配时绝不应调用 Replace")
		}
	}
	got, _ := os.ReadFile(target)
	if string(got) != "old-binary" {
		t.Error("旧二进制不应被改动")
	}
}

func TestRun_ForceReinstallsSameVersion(t *testing.T) {
	target := setupRun(t, "1.2.0")
	payload := "reinstalled"
	rel, archive, checksums := makeRelease(t, "1.2.0", payload)
	replacer := &recordingReplacer{}

	out, err := Run(context.Background(), Options{
		Force:    true,
		Releaser: &fakeReleaser{rel: rel},
		Fetcher:  fetcherFor(rel, map[string][]byte{rel.Assets[0].Name: archive, checksumsAssetName: checksums}),
		Replacer: replacer,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !out.Installed {
		t.Error("--force 应强制安装同版本")
	}
	got, _ := os.ReadFile(target)
	if string(got) != payload {
		t.Error("强制安装后内容应为新 payload")
	}
}

func TestRun_DryRunDoesNotReplace(t *testing.T) {
	target := setupRun(t, "1.0.0")
	rel, archive, checksums := makeRelease(t, "1.2.0", "payload")
	replacer := &recordingReplacer{}

	out, err := Run(context.Background(), Options{
		DryRun:   true,
		Releaser: &fakeReleaser{rel: rel},
		Fetcher:  fetcherFor(rel, map[string][]byte{rel.Assets[0].Name: archive, checksumsAssetName: checksums}),
		Replacer: replacer,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Installed {
		t.Error("dry-run 不应标记 Installed")
	}
	for _, c := range replacer.calls {
		if c == "Replace" {
			t.Error("dry-run 不应调用 Replace")
		}
	}
	got, _ := os.ReadFile(target)
	if string(got) != "old-binary" {
		t.Error("dry-run 不应改动二进制")
	}
}

func TestRun_NoAssetForPlatform(t *testing.T) {
	setupRun(t, "1.0.0")
	rel := Release{TagName: "v1.2.0", Assets: []Asset{
		{Name: "cc-select_1.2.0_plan9_amd64.tar.gz", BrowserDownloadURL: "https://x"},
	}}

	_, err := Run(context.Background(), Options{Releaser: &fakeReleaser{rel: rel}})
	if err == nil {
		t.Fatal("无本平台产物应报错")
	}
}

// ---------- acquireLock ----------

func TestAcquireLock_MutualExclusion(t *testing.T) {
	withLockDir(t, t.TempDir())

	unlock, err := acquireLock()
	if err != nil {
		t.Fatalf("首次加锁: %v", err)
	}
	if _, err := acquireLock(); err == nil {
		t.Fatal("持锁期间第二次加锁应被拒绝")
	}
	unlock()
	if _, err := acquireLock(); err != nil {
		t.Fatalf("释放后应能再次加锁: %v", err)
	}
}

func TestAcquireLock_StaleLockStolen(t *testing.T) {
	dir := t.TempDir()
	withLockDir(t, dir)

	// 造一个 20 分钟前的陈旧锁。
	lockPath := filepath.Join(dir, "update.lock")
	if err := os.WriteFile(lockPath, []byte("99999\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-20 * time.Minute)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}

	unlock, err := acquireLock()
	if err != nil {
		t.Fatalf("陈旧锁应被窃取: %v", err)
	}
	unlock()
}

func TestAcquireLock_NoDirSkipsLock(t *testing.T) {
	old := lockDirFn
	lockDirFn = func() (string, error) { return "", fmt.Errorf("no home") }
	t.Cleanup(func() { lockDirFn = old })

	unlock, err := acquireLock()
	if err != nil {
		t.Fatalf("锁目录不可用应降级为无锁: %v", err)
	}
	unlock()
}
