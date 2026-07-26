package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cc-select/cc-select/internal/i18n"
)

// httpFetcher 是默认 AssetFetcher：带 ctx 的 GET，限流读取，校验状态码。
type httpFetcher struct {
	client *http.Client
}

func defaultFetcher() AssetFetcher {
	return httpFetcher{client: defaultHTTPClient}
}

func (h httpFetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, i18n.Ew("errors.update.downloadFailed", err, url)
	}
	req.Header.Set("User-Agent", "cc-select-updater")
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, i18n.Ew("errors.update.downloadFailed", err, url)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, i18n.Ew("errors.update.downloadFailed", fmt.Errorf("HTTP %d", resp.StatusCode), url)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAssetSize))
	if err != nil {
		return nil, i18n.Ew("errors.update.downloadFailed", err, url)
	}
	return data, nil
}

// FetchAsset 下载 asset 与 checksums.txt，返回 asset 字节与期望的 SHA-256。
// 只做下载与校验值提取，不做 SHA-256 比对——那是 VerifyAsset 的职责，
// 拆开是为了让调用方（Run）能在两个阶段之间输出准确的进度日志。
func FetchAsset(ctx context.Context, fetcher AssetFetcher, rel Release, assetName string) (assetBytes []byte, expectedSum string, err error) {
	var asset *Asset
	checksumsURL := ""
	for i := range rel.Assets {
		switch rel.Assets[i].Name {
		case assetName:
			asset = &rel.Assets[i]
		case checksumsAssetName:
			checksumsURL = rel.Assets[i].BrowserDownloadURL
		}
	}
	if asset == nil {
		// resolveLatest 已按平台匹配过；走到这里说明 release 资产列表异常。
		return nil, "", i18n.E("errors.update.noAssetForPlatform", runtime.GOOS, runtime.GOARCH)
	}
	if checksumsURL == "" {
		// checksums.txt 不在 assets 列表时按约定路径拼接（goreleaser 总是产出，此为兜底）。
		checksumsURL = fmt.Sprintf("%s/%s/releases/download/%s/%s",
			githubDownloadBase, GitHubRepo, rel.TagName, checksumsAssetName)
	}

	assetBytes, err = fetcher.Fetch(ctx, asset.BrowserDownloadURL)
	if err != nil {
		return nil, "", err
	}
	checksumsBytes, err := fetcher.Fetch(ctx, checksumsURL)
	if err != nil {
		return nil, "", err
	}

	expectedSum, found := ParseChecksums(checksumsBytes, assetName)
	if !found {
		return nil, "", i18n.E("errors.update.checksumMissing", assetName)
	}
	return assetBytes, expectedSum, nil
}

// VerifyAsset 比对 asset 字节的 SHA-256 与期望值。
// 校验失败时返回错误且旧二进制不动——此时尚未写入安装路径附近。
func VerifyAsset(assetBytes []byte, assetName, expectedSum string) error {
	if !VerifySHA256(assetBytes, expectedSum) {
		return i18n.E("errors.update.checksumMismatch", assetName, expectedSum, sha256Hex(assetBytes))
	}
	return nil
}

// Download 下载并校验（= FetchAsset + VerifyAsset 的组合便捷入口）。
// 移植 scripts/install.sh verify_checksum / scripts/install.ps1 Verify-Checksum。
func Download(ctx context.Context, fetcher AssetFetcher, rel Release, assetName string) ([]byte, error) {
	assetBytes, expectedSum, err := FetchAsset(ctx, fetcher, rel, assetName)
	if err != nil {
		return nil, err
	}
	if err := VerifyAsset(assetBytes, assetName, expectedSum); err != nil {
		return nil, err
	}
	return assetBytes, nil
}

// ParseChecksums 从 checksums.txt 内容中提取 assetName 的 SHA-256。
// 格式（goreleaser 产出，同 install.sh:66 / install.ps1:89）："<sha256>  <assetname>"，
// 两字段以空白分隔，asset 是 basename。找不到返回 found=false。
func ParseChecksums(checksums []byte, assetName string) (expected string, found bool) {
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue // 跳过空行/异常行
		}
		if fields[1] == assetName {
			return strings.ToLower(fields[0]), true
		}
	}
	return "", false
}

// VerifySHA256 报告 data 的 SHA-256（小写 hex）是否等于 expected（大小写不敏感）。
func VerifySHA256(data []byte, expected string) bool {
	return strings.EqualFold(sha256Hex(data), expected)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// extractBinary 把归档根目录的 cc-select/cc-select.exe 解压到 targetDir 旁的
// temp 目录（与 target 同文件系统，保证后续 os.Rename 原子），返回路径与 cleanup。
// 归档格式由 assetName 后缀决定：.zip / .tar.gz。
func extractBinary(archiveBytes []byte, assetName, targetDir string) (binPath string, cleanup func(), err error) {
	binName := "cc-select"
	if strings.HasSuffix(assetName, ".zip") {
		binName = "cc-select.exe"
	}

	var data []byte
	if strings.HasSuffix(assetName, ".zip") {
		data, err = unzipBinary(archiveBytes, binName)
	} else {
		data, err = untarGzBinary(archiveBytes, binName)
	}
	if err != nil {
		return "", nil, err
	}

	tmpDir, err := os.MkdirTemp(targetDir, ".cc-select-update-*")
	if err != nil {
		return "", nil, i18n.Ew("errors.update.replaceFailed", err)
	}
	cleanup = func() { _ = os.RemoveAll(tmpDir) }

	binPath = filepath.Join(tmpDir, binName)
	if err := os.WriteFile(binPath, data, 0o755); err != nil {
		cleanup()
		return "", nil, i18n.Ew("errors.update.replaceFailed", err)
	}
	return binPath, cleanup, nil
}

// untarGzBinary 从 tar.gz 字节中提取名为 binName 的常规文件（容忍 "./cc-select" 前缀）。
// 归档损坏用 archiveInvalid；归档完好但缺二进制用 archiveNoBinary。
func untarGzBinary(archiveBytes []byte, binName string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archiveBytes))
	if err != nil {
		return nil, i18n.Ew("errors.update.archiveInvalid", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, i18n.Ew("errors.update.archiveInvalid", err)
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == binName {
			return io.ReadAll(tr)
		}
	}
	return nil, i18n.E("errors.update.archiveNoBinary")
}

// unzipBinary 从 zip 字节中提取名为 binName 的文件。
func unzipBinary(archiveBytes []byte, binName string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		return nil, i18n.Ew("errors.update.archiveInvalid", err)
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(f.Name) != binName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, i18n.Ew("errors.update.archiveInvalid", err)
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, i18n.E("errors.update.archiveNoBinary")
}
