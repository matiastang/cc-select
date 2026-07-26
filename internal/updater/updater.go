// Package updater 从 GitHub Releases 自更新 cc-select 二进制。
//
// 设计镜像 internal/rcinteg：
//   - 引擎与 IO 解耦：Check/Run 通过接口（Releaser/AssetFetcher/Replacer）编排，
//     测试可注入 fake 与 httptest.Server。
//   - 纯函数（compareVersions/AssetName/ParseChecksums/DetectManager）不碰 IO，可单测。
//   - 全程 i18n.T/E/Ew。
//   - 拒绝自更新 dev 构建与 Homebrew/Scoop 安装（避免包管理器元数据漂移）。
//
// 替换后运行中进程继续服务（Unix 持有旧 inode；Windows 走 renamed-.old），
// 新版本在下次启动时生效——本包不做 re-exec（重启策略是应用层关注点）。
package updater

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/cc-select/cc-select/internal/i18n"
)

// GitHubRepo 是 Release 仓库。注意与 Go module path（github.com/cc-select/cc-select）
// 不同——API 与下载 URL 必须用本值。
const GitHubRepo = "matiastang/cc-select"

// checksumsAssetName 是 goreleaser 产出的校验文件名（.goreleaser.yaml checksum.name_template）。
const checksumsAssetName = "checksums.txt"

// lockStaleAge 是 update.lock 的陈旧阈值：超过则认为持锁进程已死，窃取锁。
const lockStaleAge = 10 * time.Minute

// maxAssetSize 限制下载体积（防异常/恶意无限流）。release 二进制约 10-30MB。
const maxAssetSize = 512 << 20

// 可注入的包变量：测试覆盖指向 httptest.Server（= rcinteg.profileProbe 模式）。
var (
	githubAPIBase      = "https://api.github.com"
	githubDownloadBase = "https://github.com"
)

// defaultHTTPClient 使用保守超时（GitHub API 可能慢）。Run 的 ctx 仍可在中途取消。
var defaultHTTPClient = &http.Client{Timeout: 30 * time.Second}

// Options 控制 Check/Run 行为，所有字段可选。
type Options struct {
	AllowPrerelease bool             // latest 查询包含预发布版本
	Force           bool             // 即使 HasUpdate=false 也安装（修复损坏二进制用）
	DryRun          bool             // 下载+校验+解压但不替换
	GitHubToken     string           // 可选；提升 API 限流（60→5000/hr）
	Releaser        Releaser         // nil => 默认 GitHub releaser
	Fetcher         AssetFetcher     // nil => 默认 HTTP fetcher
	Replacer        Replacer         // nil => defaultReplacer()（OS 相关）
	Logf            func(msg string) // 可选进度日志（CLI 打 stderr；Web 不传）
}

// Release 描述一个 GitHub release，仅含消费的字段。
type Release struct {
	TagName    string // "v1.2.3"
	Name       string
	Prerelease bool
	Body       string // release notes（markdown）
	HTMLURL    string
	Assets     []Asset
}

// Asset 是 release 挂载的一个可下载文件。
type Asset struct {
	Name               string
	BrowserDownloadURL string
	Size               int64
}

// Result 是 Check 的结果（无 IO 写入）。
type Result struct {
	Current      string
	Latest       string // bare 版本，无 'v'
	HasUpdate    bool
	DevBuild     bool   // version.Version == "dev" 短路标记
	AssetName    string // 匹配当前平台的 asset 名；空 = 无本平台产物
	ReleaseNotes string
	HTMLURL      string
}

// Outcome 是 Run 的结果（完整流水线）。
type Outcome struct {
	Result
	Installed   bool
	FromVersion string
	ToVersion   string
	Message     string // i18n 格式化，用户可读
}

// Releaser 抽象 GitHub release 元数据获取。
type Releaser interface {
	Latest(ctx context.Context, allowPrerelease bool) (Release, error)
}

// AssetFetcher 抽象 HTTP 字节下载（asset + checksums.txt）。
type AssetFetcher interface {
	Fetch(ctx context.Context, url string) ([]byte, error)
}

// Replacer 抽象原子二进制替换（Unix rename / Windows .old dance）。
// 抽为接口使 Windows 路径可在 Unix CI 用 fake 单测调用序列。
type Replacer interface {
	// Replace 原子地把 newBinaryPath 安装到运行 exe 的位置，并做备份（Unix .bak / Windows .old）。
	Replace(newBinaryPath, targetPath string) error
	// CleanupStaleBackup 清理上次 run 残留的 .old/.bak；best-effort。
	CleanupStaleBackup(targetPath string) error
}

// RefusedError 表示自更新被策略拒绝（dev 构建 / 包管理器安装 / 目录不可写）。
// 拒绝不是失败：CLI 打印消息并 exit 0；Web 返回 409 + kind 供前端精确引导。
type RefusedError struct {
	Kind string // KindDev / KindHomebrew / KindScoop / KindNotWritable
	Msg  string
}

// 拒绝原因枚举（同时作为 Web 409 响应的 kind 字段）。
const (
	KindDev         = "dev"
	KindHomebrew    = "homebrew"
	KindScoop       = "scoop"
	KindNotWritable = "notWritable"
)

func (e *RefusedError) Error() string { return e.Msg }

// log 向可选的进度日志写一条消息。
func (o Options) log(msg string) {
	if o.Logf != nil {
		o.Logf(msg)
	}
}

// Run 是完整的 检查→下载→校验→替换 流水线（CLI 与 Web 共用）。
// 对应 rcinteg.Install 的角色。
func Run(ctx context.Context, opts Options) (Outcome, error) {
	// 1. 安装上下文检测：dev / Homebrew / Scoop / 不可写 → 拒绝。
	dctx, err := detectContextFn()
	if err != nil {
		return Outcome{}, err
	}
	if rerr := refuseIfNeeded(dctx); rerr != nil {
		return Outcome{}, rerr
	}

	// 2. 跨进程锁（单进程内由调用方的 mutex 串行，见 web updateMu）。
	unlock, err := acquireLock()
	if err != nil {
		return Outcome{}, err
	}
	defer unlock()

	// 3. 检查最新版本。
	opts.log(i18n.T("cli.update.checking"))
	res, rel, err := resolveLatest(ctx, opts)
	if err != nil {
		return Outcome{}, err
	}
	out := Outcome{Result: res, FromVersion: res.Current, ToVersion: res.Latest}

	if !res.HasUpdate && !opts.Force {
		out.Message = i18n.T("cli.update.upToDate", res.Current, res.Latest)
		return out, nil
	}
	if res.AssetName == "" {
		return out, i18n.E("errors.update.noAssetForPlatform", runtime.GOOS, runtime.GOARCH)
	}

	replacer := opts.Replacer
	if replacer == nil {
		replacer = defaultReplacer()
	}
	_ = replacer.CleanupStaleBackup(dctx.ExecPath) // best-effort，失败忽略

	fetcher := opts.Fetcher
	if fetcher == nil {
		fetcher = defaultFetcher()
	}

	// 4. 下载 asset + checksums.txt（失败时旧二进制不动——此时还没写入安装路径附近）。
	opts.log(i18n.T("cli.update.downloading", res.AssetName))
	assetBytes, expectedSum, err := FetchAsset(ctx, fetcher, rel, res.AssetName)
	if err != nil {
		return out, err
	}

	// 5. 校验 SHA-256（不匹配则保留旧二进制）。
	opts.log(i18n.T("cli.update.verifying"))
	if err := VerifyAsset(assetBytes, res.AssetName, expectedSum); err != nil {
		return out, err
	}

	// 6. 解压到安装目录旁的 temp（保证与 target 同文件系统，rename 才原子）。
	binPath, cleanup, err := extractBinary(assetBytes, res.AssetName, filepath.Dir(dctx.ExecPath))
	if err != nil {
		return out, err
	}
	defer cleanup()

	// 7. dry-run：到此为止，不替换。
	if opts.DryRun {
		out.Message = i18n.T("cli.update.dryRunOk", res.AssetName)
		return out, nil
	}

	// 8. 原子替换。
	opts.log(i18n.T("cli.update.installing"))
	if err := replacer.Replace(binPath, dctx.ExecPath); err != nil {
		return out, err
	}

	out.Installed = true
	out.Message = i18n.T("cli.update.installed", res.Current, res.Latest)
	return out, nil
}

// refuseIfNeeded 按策略拒绝：包管理器安装 > dev 构建 > 目录不可写。
// release 版被 brew/scoop 装的场景最常见，故 Manager 检测优先于 dev。
func refuseIfNeeded(dctx Context) error {
	switch dctx.Manager {
	case ManagerHomebrew:
		return &RefusedError{Kind: KindHomebrew, Msg: i18n.T("errors.update.managedHomebrew")}
	case ManagerScoop:
		return &RefusedError{Kind: KindScoop, Msg: i18n.T("errors.update.managedScoop")}
	}
	if dctx.IsDev {
		return &RefusedError{Kind: KindDev, Msg: i18n.T("errors.update.devBuild")}
	}
	if !dctx.Writable {
		return &RefusedError{Kind: KindNotWritable, Msg: i18n.T("errors.update.notWritable", filepath.Dir(dctx.ExecPath))}
	}
	return nil
}

// acquireLock 用 <lockDir>/update.lock 做跨进程互斥（O_CREATE|O_EXCL）。
// 锁文件存在且 mtime 在 lockStaleAge 内 → 另一个更新进行中，拒绝；
// 超过阈值 → 持锁进程被认为已死，窃取锁。
// 锁不可用时（无 home 目录 / 只读文件系统）降级为无锁继续——锁是防护而非正确性依赖。
func acquireLock() (func(), error) {
	dir, err := lockDirFn()
	if err != nil {
		return func() {}, nil // best-effort：拿不到目录就跳过锁
	}
	lockPath := filepath.Join(dir, "update.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if !os.IsExist(err) {
			return func() {}, nil // 其他错误（只读 fs 等）→ 跳过锁
		}
		// 锁已存在：检查是否陈旧。
		stale := false
		if st, serr := os.Stat(lockPath); serr == nil {
			stale = time.Since(st.ModTime()) > lockStaleAge
		}
		if !stale {
			return nil, i18n.E("errors.update.locked")
		}
		_ = os.Remove(lockPath) // 窃取陈旧锁
		f, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			if os.IsExist(err) {
				return nil, i18n.E("errors.update.locked")
			}
			return func() {}, nil
		}
	}
	fmt.Fprintf(f, "%d\n", os.Getpid())
	_ = f.Close()
	return func() { _ = os.Remove(lockPath) }, nil
}

// lockDirFn 返回锁文件所在目录（默认 ~/.cc-select）。测试可注入指向 t.TempDir()。
var lockDirFn = defaultLockDir

func defaultLockDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".cc-select")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}
