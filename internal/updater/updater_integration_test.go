//go:build integration

package updater

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestRun_FullPipeline_HTTPServer 是 updater 的端到端集成测试：
// 真实 HTTP（httptest.Server 模拟 GitHub release）+ 默认 releaser/fetcher +
// 当前 OS 的真实 Replacer（Unix rename / Windows .old dance），
// 断言 target 二进制被替换为新 payload，且留下平台对应的备份（.bak / .old）。
//
// 对应 rcinteg.TestInstall_Zsh_AppendsThenNoop 的角色。
func TestRun_FullPipeline_HTTPServer(t *testing.T) {
	withVersion(t, "1.0.0")
	dir := t.TempDir()
	target := filepath.Join(dir, binNameForOS())
	if err := os.WriteFile(target, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	withDetectContext(t, Context{ExecPath: target, IsDev: false, Manager: ManagerNone, Writable: true})
	withLockDir(t, t.TempDir())

	payload := "stub payload for " + runtime.GOOS + "/" + runtime.GOARCH
	assetName, err := AssetName("1.2.0", runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("AssetName: %v", err)
	}
	archive := buildArchive(t, assetName, binNameForOS(), payload)
	checksums := fmt.Sprintf("%s  %s\n", sha256Hex(archive), assetName)

	mux := http.NewServeMux()
	// 两阶段：handler 闭包引用 srvURL 变量，server 起来后回填（release JSON 里的
	// browser_download_url 需要完整 URL）。
	var srvURL string
	mux.HandleFunc("/repos/"+GitHubRepo+"/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
		  "tag_name": "v1.2.0",
		  "body": "notes",
		  "html_url": "https://example.test/v1.2.0",
		  "assets": [
		    {"name": %q, "browser_download_url": %q, "size": %d},
		    {"name": "checksums.txt", "browser_download_url": %q, "size": %d}
		  ]
		}`, assetName, srvURL+"/dl/"+assetName, len(archive),
			srvURL+"/dl/checksums.txt", len(checksums))
	})
	mux.HandleFunc("/dl/"+assetName, func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	})
	mux.HandleFunc("/dl/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(checksums))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL
	withGitHubBases(t, srv.URL)

	// 全部默认实现：真 HTTP releaser/fetcher + 真 OS replacer。
	out, err := Run(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !out.Installed || out.ToVersion != "1.2.0" {
		t.Errorf("Outcome 错误: %+v", out)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("读取替换后的 target: %v", err)
	}
	if string(got) != payload {
		t.Errorf("target 内容 = %q, want %q", got, payload)
	}

	// 平台对应的备份工件。
	if runtime.GOOS == "windows" {
		if _, err := os.Stat(target + ".old"); err != nil {
			t.Errorf("Windows 替换后应留 .old 备份: %v", err)
		}
	} else {
		bak, err := os.ReadFile(target + ".bak")
		if err != nil {
			t.Errorf("Unix 替换后应留 .bak 备份: %v", err)
		} else if string(bak) != "old-binary" {
			t.Errorf(".bak 内容 = %q, want old-binary", bak)
		}
	}
}
