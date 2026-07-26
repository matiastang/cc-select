package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/version"
)

// Check 查询最新 release 并与当前二进制版本比较（只读，不做任何写入）。
// dev 构建短路：返回 Result{Current:"dev", DevBuild:true}，不访问网络。
// 注意：Check 不拒绝 Homebrew/Scoop——它只报告「有没有新版」，拒绝是 Run 的职责
// （brew 用户同样想知道上游有没有新版）。
func Check(ctx context.Context, opts Options) (Result, error) {
	res, _, err := resolveLatest(ctx, opts)
	return res, err
}

// resolveLatest 是 Check 与 Run 共用的内部实现：额外返回完整 Release（Run 下载要用 Assets）。
func resolveLatest(ctx context.Context, opts Options) (Result, Release, error) {
	if IsDevBuild() {
		return Result{Current: version.Version, DevBuild: true}, Release{}, nil
	}
	r := opts.Releaser
	if r == nil {
		r = defaultReleaser(opts.GitHubToken)
	}
	rel, err := r.Latest(ctx, opts.AllowPrerelease)
	if err != nil {
		return Result{}, Release{}, err
	}
	latest := stripLeadingV(rel.TagName)
	res := Result{
		Current:      version.Version,
		Latest:       latest,
		HasUpdate:    compareVersions(latest, version.Version) > 0,
		ReleaseNotes: rel.Body,
		HTMLURL:      rel.HTMLURL,
	}
	// 选当前平台的 asset；找不到时 AssetName 留空（由调用方决定提示还是报错）。
	want, err := AssetName(latest, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return res, rel, nil
	}
	for _, a := range rel.Assets {
		if a.Name == want {
			res.AssetName = a.Name
			break
		}
	}
	return res, rel, nil
}

// IsDevBuild 报告当前二进制是否为开发构建（make dev / go build，无 ldflags 注入）。
func IsDevBuild() bool { return version.Version == "dev" }

// GitHubTokenFromEnv 读 GITHUB_TOKEN / GH_TOKEN（提升 GitHub API 限流额度，60→5000/hr）。
// token 仅经 HTTPS 发往 api.github.com。CLI 与 Web 共用此函数。
func GitHubTokenFromEnv() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GH_TOKEN")
}

// AssetName 构造 goreleaser 约定的归档名：cc-select_<bare-version>_<goos>_<goarch>.<ext>，
// windows 用 .zip，其余用 .tar.gz（.goreleaser.yaml name_template + format_overrides）。
// version 是 bare 版本（无 'v'）。不支持的平台（windows/arm64）返回错误。
func AssetName(version, goos, goarch string) (string, error) {
	if !SupportedPlatform(goos, goarch) {
		return "", i18n.E("errors.update.unsupportedPlatform", goos, goarch)
	}
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("cc-select_%s_%s_%s%s", version, goos, goarch, ext), nil
}

// SupportedPlatform 报告 (goos, goarch) 是否有 release 产物。
// 与 .goreleaser.yaml 构建矩阵一致：darwin/linux amd64+arm64、windows 仅 amd64。
func SupportedPlatform(goos, goarch string) bool {
	switch goos {
	case "darwin", "linux":
		return goarch == "amd64" || goarch == "arm64"
	case "windows":
		return goarch == "amd64" // windows/arm64 不构建（.goreleaser.yaml ignore）
	}
	return false
}

// stripLeadingV 去掉单个前导 'v'/'V'（tag_name "v1.2.3" → "1.2.3"）。
func stripLeadingV(tag string) string {
	if len(tag) > 0 && (tag[0] == 'v' || tag[0] == 'V') {
		return tag[1:]
	}
	return tag
}

// semverParts 是解析后的 semver 分量。
type semverParts struct {
	major, minor, patch int
	pre                 string // prerelease 段（"beta.1"）；空 = 稳定版
}

// parseSemver 解析 "1.2.3-beta.1+build"（可带前导 v）。build metadata（+ 后）忽略。
// 宽松处理：允许省略 minor/patch（"1" / "1.2" 视为 1.0.0 / 1.2.0）。
func parseSemver(v string) (semverParts, bool) {
	v = stripLeadingV(strings.TrimSpace(v))
	if v == "" {
		return semverParts{}, false
	}
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	var p semverParts
	if i := strings.IndexByte(v, '-'); i >= 0 {
		p.pre = v[i+1:]
		v = v[:i]
	}
	nums := strings.Split(v, ".")
	if len(nums) > 3 {
		return semverParts{}, false
	}
	fields := []*int{&p.major, &p.minor, &p.patch}
	for i, n := range nums {
		num, err := strconv.Atoi(n)
		if err != nil || num < 0 {
			return semverParts{}, false
		}
		*fields[i] = num
	}
	return p, true
}

// compareVersions 比较两个版本：a<b 返回 -1，相等 0，a>b 返回 +1。
// 自实现（约 50 行）而非引 x/mod/semver：零新依赖，且 x/mod 强制 'v' 前缀与 bare 约定冲突。
// 不可解析的版本（如 "dev"）视为小于任何可解析版本；两者都不可解析时按字典序兜底。
func compareVersions(a, b string) int {
	pa, oka := parseSemver(a)
	pb, okb := parseSemver(b)
	if !oka && !okb {
		return strings.Compare(a, b)
	}
	if !oka {
		return -1
	}
	if !okb {
		return 1
	}
	if pa.major != pb.major {
		return cmpInt(pa.major, pb.major)
	}
	if pa.minor != pb.minor {
		return cmpInt(pa.minor, pb.minor)
	}
	if pa.patch != pb.patch {
		return cmpInt(pa.patch, pb.patch)
	}
	// SemVer §11：无 prerelease 的稳定版 > 有 prerelease 的同号版本。
	if pa.pre == "" && pb.pre == "" {
		return 0
	}
	if pa.pre == "" {
		return 1
	}
	if pb.pre == "" {
		return -1
	}
	return comparePrerelease(pa.pre, pb.pre)
}

// comparePrerelease 按 SemVer §11 比较 prerelease 段：逐段比，
// 数值段按数值、字母段按字典序、数值段 < 字母段；前缀相同则段少者小。
func comparePrerelease(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		an, aerr := strconv.Atoi(as[i])
		bn, berr := strconv.Atoi(bs[i])
		switch {
		case aerr == nil && berr == nil:
			if an != bn {
				return cmpInt(an, bn)
			}
		case aerr == nil:
			return -1 // 数值段 < 字母段
		case berr == nil:
			return 1
		default:
			if c := strings.Compare(as[i], bs[i]); c != 0 {
				return c
			}
		}
	}
	return cmpInt(len(as), len(bs))
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// releaseJSON 是 GitHub API 响应的解码结构（仅消费字段）。
type releaseJSON struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
	Body       string `json:"body"`
	HTMLURL    string `json:"html_url"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

func (r releaseJSON) toRelease() Release {
	rel := Release{
		TagName:    r.TagName,
		Name:       r.Name,
		Prerelease: r.Prerelease,
		Body:       r.Body,
		HTMLURL:    r.HTMLURL,
		Assets:     make([]Asset, 0, len(r.Assets)),
	}
	for _, a := range r.Assets {
		rel.Assets = append(rel.Assets, Asset{
			Name:               a.Name,
			BrowserDownloadURL: a.BrowserDownloadURL,
			Size:               a.Size,
		})
	}
	return rel
}

// githubReleaser 是默认 Releaser：直连 GitHub API。
type githubReleaser struct {
	client *http.Client
	token  string
}

func defaultReleaser(token string) Releaser {
	return githubReleaser{client: defaultHTTPClient, token: token}
}

// Latest 取最新 release。默认走 /releases/latest（该端点本身已排除 prerelease）；
// allowPrerelease 时改走 /releases 列表，取首个非 draft 且版本号最大者。
func (g githubReleaser) Latest(ctx context.Context, allowPrerelease bool) (Release, error) {
	if !allowPrerelease {
		var rj releaseJSON
		if err := g.getJSON(ctx, fmt.Sprintf("%s/repos/%s/releases/latest", githubAPIBase, GitHubRepo), &rj); err != nil {
			return Release{}, err
		}
		return rj.toRelease(), nil
	}
	var list []releaseJSON
	if err := g.getJSON(ctx, fmt.Sprintf("%s/repos/%s/releases?per_page=20", githubAPIBase, GitHubRepo), &list); err != nil {
		return Release{}, err
	}
	var candidates []releaseJSON
	for _, rj := range list {
		if !rj.Draft {
			candidates = append(candidates, rj)
		}
	}
	if len(candidates) == 0 {
		return Release{}, i18n.E("errors.update.noRelease")
	}
	sort.Slice(candidates, func(i, j int) bool {
		return compareVersions(candidates[i].TagName, candidates[j].TagName) > 0
	})
	return candidates[0].toRelease(), nil
}

// getJSON 发带 UA 的 GET（GitHub API 无 User-Agent 会 403）并解码 JSON。
// 检测限流（403 + X-RateLimit-Remaining: 0）给出可操作的错误。
func (g githubReleaser) getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return i18n.Ew("errors.update.network", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "cc-select-updater")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return i18n.Ew("errors.update.network", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
		return i18n.E("errors.update.rateLimited")
	}
	if resp.StatusCode != http.StatusOK {
		return i18n.Ew("errors.update.network", fmt.Errorf("HTTP %d", resp.StatusCode))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return i18n.Ew("errors.update.network", err)
	}
	return nil
}
