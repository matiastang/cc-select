package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestAssetName_AllPlatforms(t *testing.T) {
	cases := []struct {
		goos, goarch, want string
	}{
		{"darwin", "amd64", "cc-select_1.2.3_darwin_amd64.tar.gz"},
		{"darwin", "arm64", "cc-select_1.2.3_darwin_arm64.tar.gz"},
		{"linux", "amd64", "cc-select_1.2.3_linux_amd64.tar.gz"},
		{"linux", "arm64", "cc-select_1.2.3_linux_arm64.tar.gz"},
		{"windows", "amd64", "cc-select_1.2.3_windows_amd64.zip"},
	}
	for _, c := range cases {
		got, err := AssetName("1.2.3", c.goos, c.goarch)
		if err != nil {
			t.Errorf("AssetName(%s/%s): %v", c.goos, c.goarch, err)
			continue
		}
		if got != c.want {
			t.Errorf("AssetName(%s/%s) = %q, want %q", c.goos, c.goarch, got, c.want)
		}
	}
}

func TestAssetName_Unsupported(t *testing.T) {
	for _, p := range [][2]string{{"windows", "arm64"}, {"freebsd", "amd64"}, {"linux", "386"}} {
		if _, err := AssetName("1.2.3", p[0], p[1]); err == nil {
			t.Errorf("AssetName(%s/%s) 应报错", p[0], p[1])
		}
		if SupportedPlatform(p[0], p[1]) {
			t.Errorf("SupportedPlatform(%s/%s) 应为 false", p[0], p[1])
		}
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.0", 0},
		{"v1.2.3", "1.2.3", 0},
		{"V1.2.3", "1.2.3", 0},
		{"2.0.0", "10.0.0", -1},    // 数值比较而非字典序
		{"1.2", "1.2.0", 0},        // 宽松省略 minor/patch
		{"1.0.0", "1.0.0-beta", 1}, // 稳定版 > 预发布
		{"1.0.0-beta", "1.0.0", -1},
		{"1.0.0-beta.1", "1.0.0-beta.2", -1},
		{"1.0.0-alpha", "1.0.0-alpha.1", -1}, // 段少 < 段多
		{"1.0.0-alpha", "1.0.0-beta", -1},    // 字母段字典序
		{"1.0.0-1", "1.0.0-alpha", -1},       // 数值段 < 字母段
		{"1.0.0+build1", "1.0.0+build2", 0},  // build metadata 忽略
		{"dev", "1.0.0", -1},                 // 不可解析 < 可解析
		{"dev", "dev", 0},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestParseSemver(t *testing.T) {
	p, ok := parseSemver("v1.2.3-beta.1+build5")
	if !ok {
		t.Fatal("应解析成功")
	}
	if p.major != 1 || p.minor != 2 || p.patch != 3 || p.pre != "beta.1" {
		t.Errorf("解析结果错误: %+v", p)
	}
	for _, bad := range []string{"", "1.2.3.4", "a.b.c", "1.-2.3"} {
		if _, ok := parseSemver(bad); ok {
			t.Errorf("parseSemver(%q) 应失败", bad)
		}
	}
}

func TestStripLeadingV(t *testing.T) {
	if stripLeadingV("v1.2.3") != "1.2.3" || stripLeadingV("V1.2.3") != "1.2.3" {
		t.Error("应去前导 v/V")
	}
	if stripLeadingV("1.2.3") != "1.2.3" || stripLeadingV("version") != "ersion" {
		t.Error("无前导 v 或非常规串")
	}
}

func TestParseChecksums(t *testing.T) {
	content := "aaa111  cc-select_1.2.0_linux_amd64.tar.gz\n" +
		"BBB222  cc-select_1.2.0_windows_amd64.zip\n" +
		"\n" + // 空行应跳过
		"malformed-line\n"
	sum, found := ParseChecksums([]byte(content), "cc-select_1.2.0_windows_amd64.zip")
	if !found || sum != "bbb222" { // 统一小写
		t.Errorf("got (%q,%v), want (bbb222,true)", sum, found)
	}
	if _, found := ParseChecksums([]byte(content), "no-such-asset"); found {
		t.Error("缺失 asset 应 found=false")
	}
}

func TestVerifySHA256(t *testing.T) {
	data := []byte("hello")
	// echo -n hello | sha256sum
	if !VerifySHA256(data, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824") {
		t.Error("已知向量应通过")
	}
	if VerifySHA256(data, "0000") {
		t.Error("错误校验值应失败")
	}
	if !VerifySHA256(data, strings.ToUpper("2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824")) {
		t.Error("大小写应不敏感")
	}
}

func TestCheck_DevBuildShortCircuit(t *testing.T) {
	withVersion(t, "dev")
	rel := &fakeReleaser{}
	res, err := Check(context.Background(), Options{Releaser: rel})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !res.DevBuild || res.HasUpdate || res.Current != "dev" {
		t.Errorf("dev 短路结果错误: %+v", res)
	}
	if rel.calls != 0 {
		t.Error("dev 短路不应访问网络（Releaser 不应被调用）")
	}
}

func TestCheck_HasUpdate(t *testing.T) {
	withVersion(t, "1.0.0")
	rel, _, _ := makeRelease(t, "1.2.0", "payload")
	res, err := Check(context.Background(), Options{Releaser: &fakeReleaser{rel: rel}})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !res.HasUpdate || res.Current != "1.0.0" || res.Latest != "1.2.0" {
		t.Errorf("结果错误: %+v", res)
	}
	wantAsset, _ := AssetName("1.2.0", runtime.GOOS, runtime.GOARCH)
	if res.AssetName != wantAsset {
		t.Errorf("AssetName = %q, want %q", res.AssetName, wantAsset)
	}
}

func TestCheck_UpToDate(t *testing.T) {
	withVersion(t, "1.2.0")
	rel, _, _ := makeRelease(t, "1.2.0", "payload")
	res, err := Check(context.Background(), Options{Releaser: &fakeReleaser{rel: rel}})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.HasUpdate {
		t.Errorf("同版本不应 HasUpdate: %+v", res)
	}
}

func TestCheck_NoMatchingAsset(t *testing.T) {
	withVersion(t, "1.0.0")
	// release 只有其它平台的 asset → AssetName 空但 HasUpdate 仍为 true。
	rel := Release{
		TagName: "v1.2.0",
		Assets:  []Asset{{Name: "cc-select_1.2.0_plan9_amd64.tar.gz", BrowserDownloadURL: "https://x"}},
	}
	res, err := Check(context.Background(), Options{Releaser: &fakeReleaser{rel: rel}})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !res.HasUpdate || res.AssetName != "" {
		t.Errorf("结果错误: %+v", res)
	}
}

// ---------- githubReleaser（httptest，回环无外部依赖） ----------

func TestGithubReleaser_Latest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("GitHub API 请求必须带 User-Agent")
		}
		if !strings.HasSuffix(r.URL.Path, "/releases/latest") {
			t.Errorf("意外路径: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v2.0.0",
			"body":     "notes",
			"html_url": "https://example.test/v2",
			"assets": []map[string]any{
				{"name": "cc-select_2.0.0_linux_amd64.tar.gz", "browser_download_url": "https://dl/x", "size": 10},
			},
		})
	}))
	defer srv.Close()
	withGitHubBases(t, srv.URL)

	rel, err := defaultReleaser("").Latest(context.Background(), false)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if rel.TagName != "v2.0.0" || len(rel.Assets) != 1 || rel.Assets[0].Size != 10 {
		t.Errorf("解析错误: %+v", rel)
	}
}

func TestGithubReleaser_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	withGitHubBases(t, srv.URL)

	_, err := defaultReleaser("").Latest(context.Background(), false)
	if err == nil || !strings.Contains(err.Error(), "rateLimited") && !strings.Contains(err.Error(), "rate limit") {
		// i18n 缺键时返回 key 本身，含 "rateLimited"。
		t.Errorf("应返回限流错误, got %v", err)
	}
}

func TestGithubReleaser_PrereleaseList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 列表含 draft（应过滤）+ 旧版 + 更新的 prerelease，按版本降序取第一。
		json.NewEncoder(w).Encode([]map[string]any{
			{"tag_name": "v1.5.0", "draft": false},
			{"tag_name": "v2.0.0-beta.1", "draft": false, "prerelease": true},
			{"tag_name": "v9.9.9", "draft": true}, // draft 必须被过滤
		})
	}))
	defer srv.Close()
	withGitHubBases(t, srv.URL)

	rel, err := defaultReleaser("").Latest(context.Background(), true)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if rel.TagName != "v2.0.0-beta.1" {
		t.Errorf("应取版本最大的非 draft, got %s", rel.TagName)
	}
}

func TestHTTPFetcher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/ok") {
			w.Write([]byte("data"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := defaultFetcher()
	data, err := f.Fetch(context.Background(), srv.URL+"/ok")
	if err != nil || string(data) != "data" {
		t.Errorf("Fetch ok: (%q,%v)", data, err)
	}
	if _, err := f.Fetch(context.Background(), srv.URL+"/missing"); err == nil {
		t.Error("404 应返回错误")
	}
}
