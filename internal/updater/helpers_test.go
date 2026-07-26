package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/cc-select/cc-select/internal/version"
)

// ---------- 测试辅助（同包各 _test.go 共享） ----------

// withVersion 临时替换注入的版本号，测试结束恢复。
func withVersion(t *testing.T, v string) {
	t.Helper()
	old := version.Version
	version.Version = v
	t.Cleanup(func() { version.Version = old })
}

// withDetectContext 注入安装上下文（绕过真实 os.Executable 探测）。
func withDetectContext(t *testing.T, c Context) {
	t.Helper()
	old := detectContextFn
	detectContextFn = func() (Context, error) { return c, nil }
	t.Cleanup(func() { detectContextFn = old })
}

// withLockDir 把锁目录指到 temp，避免碰真实 ~/.cc-select。
func withLockDir(t *testing.T, dir string) {
	t.Helper()
	old := lockDirFn
	lockDirFn = func() (string, error) { return dir, nil }
	t.Cleanup(func() { lockDirFn = old })
}

// withGitHubBases 把 API/下载基址指到 httptest.Server。
func withGitHubBases(t *testing.T, base string) {
	t.Helper()
	oldAPI, oldDL := githubAPIBase, githubDownloadBase
	githubAPIBase, githubDownloadBase = base, base
	t.Cleanup(func() { githubAPIBase, githubDownloadBase = oldAPI, oldDL })
}

// binNameForOS 返回当前平台的归档内二进制名。
func binNameForOS() string {
	if runtime.GOOS == "windows" {
		return "cc-select.exe"
	}
	return "cc-select"
}

// buildArchive 按 assetName 后缀构造真实 tar.gz / zip，内含 binName → payload。
func buildArchive(t *testing.T, assetName, binName, payload string) []byte {
	t.Helper()
	if strings.HasSuffix(assetName, ".zip") {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		w, err := zw.Create(binName)
		if err != nil {
			t.Fatalf("zip create: %v", err)
		}
		if _, err := w.Write([]byte(payload)); err != nil {
			t.Fatalf("zip write: %v", err)
		}
		if err := zw.Close(); err != nil {
			t.Fatalf("zip close: %v", err)
		}
		return buf.Bytes()
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name: binName, Mode: 0o755, Size: int64(len(payload)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write([]byte(payload)); err != nil {
		t.Fatalf("tar write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// ---------- fake 实现 ----------

type fakeReleaser struct {
	rel   Release
	err   error
	calls int
}

func (f *fakeReleaser) Latest(context.Context, bool) (Release, error) {
	f.calls++
	return f.rel, f.err
}

type fakeFetcher struct {
	files map[string][]byte
}

func (f fakeFetcher) Fetch(_ context.Context, url string) ([]byte, error) {
	b, ok := f.files[url]
	if !ok {
		return nil, fmt.Errorf("no such url: %s", url)
	}
	return b, nil
}

// recordingReplacer 记录调用序列；Replace 时真实写入 target 以模拟替换效果。
type recordingReplacer struct {
	calls []string
	err   error
}

func (r *recordingReplacer) Replace(newBin, target string) error {
	r.calls = append(r.calls, "Replace")
	if r.err != nil {
		return r.err
	}
	data, err := os.ReadFile(newBin)
	if err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o755)
}

func (r *recordingReplacer) CleanupStaleBackup(string) error {
	r.calls = append(r.calls, "CleanupStaleBackup")
	return nil
}

// makeRelease 构造含当前平台 asset + checksums.txt 的 Release。
// 返回 Release、归档字节、checksums.txt 内容。
func makeRelease(t *testing.T, ver, payload string) (Release, []byte, []byte) {
	t.Helper()
	assetName, err := AssetName(ver, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("AssetName: %v", err)
	}
	archive := buildArchive(t, assetName, binNameForOS(), payload)
	checksums := []byte(fmt.Sprintf("%s  %s\n", sha256Hex(archive), assetName))
	rel := Release{
		TagName: "v" + ver,
		Body:    "release notes for " + ver,
		HTMLURL: "https://example.test/release/v" + ver,
		Assets: []Asset{
			{Name: assetName, BrowserDownloadURL: "https://example.test/dl/" + assetName},
			{Name: checksumsAssetName, BrowserDownloadURL: "https://example.test/dl/" + checksumsAssetName},
		},
	}
	return rel, archive, checksums
}

// fetcherFor 把 Release 的 asset/checksums URL 映射到内容。
func fetcherFor(rel Release, contents map[string][]byte) fakeFetcher {
	files := map[string][]byte{}
	for _, a := range rel.Assets {
		if c, ok := contents[a.Name]; ok {
			files[a.BrowserDownloadURL] = c
		}
	}
	return fakeFetcher{files: files}
}
