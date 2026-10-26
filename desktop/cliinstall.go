package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// 内置 CLI 安装器（US2，FR-004/FR-005；契约 contracts/desktop-api.md §3.3）。
//
// 四态语义：
//   - PATH 无 cc-select        → installed（写入首个可写目标目录）
//   - 已装同版本（本工具格式） → reinstalled（原地幂等覆盖）
//   - 已装旧版本               → upgraded（原地覆盖）
//   - 已装外部/无法识别二进制  → external-binary 冲突（409）；confirmOverwrite 后 overwritten
//   - 目标目录全部无权限       → permission 冲突（409）；unix 依序回落（/usr/local/bin → ~/.bin）
//
// 版本识别格式：`cc-select v<version>`（internal/cli 的 --version 输出）。
// 本文件消息暂用英文直串（与 web 层 writeError 兜底一致），i18n 键化记为后续增强。

const (
	conflictExternal   = "external-binary"
	conflictPermission = "permission"
)

// 测试缝（沿用 internal/web/api.go 的 var fn 模式）：单测注入假 LookPath。
var lookPathFn = exec.LookPath

type cliKind int

const (
	cliNone cliKind = iota
	cliOurs
	cliExternal
)

type installResult struct {
	InstalledPath    string `json:"installedPath"`
	InstalledVersion string `json:"installedVersion"`
	Action           string `json:"action"`
	Hint             string `json:"hint,omitempty"`
}

type installConflict struct {
	Kind    string `json:"conflict"`
	Path    string `json:"installedPath,omitempty"`
	Version string `json:"installedVersion,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

type installOptions struct {
	// TargetDirs 是候选安装目录（按序回落）；已安装场景忽略（原地升级）。
	TargetDirs       []string
	EmbeddedVersion  string
	ExtractEmbedded  func(dest string) error
	ConfirmOverwrite bool
}

// cliFileName 平台二进制名（Windows 带 .exe）。
func cliFileName() string {
	if runtime.GOOS == "windows" {
		return "cc-select.exe"
	}
	return "cc-select"
}

// parseOurVersion 识别 `cc-select v<version>` 输出；非本工具格式返回 false。
func parseOurVersion(out string) (string, bool) {
	const prefix = "cc-select v"
	if strings.HasPrefix(out, prefix) {
		return strings.TrimPrefix(out, prefix), true
	}
	return "", false
}

// probeCliVersion 执行 <bin> --version 并返回首行去换行输出。
func probeCliVersion(bin string) (string, error) {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return "", err
	}
	line := bytes.SplitN(bytes.TrimSpace(out), []byte("\n"), 2)[0]
	return string(line), nil
}

// detectInstalledCLI 探测 PATH 上的 cc-select 状态。
func detectInstalledCLI() (path, version string, kind cliKind) {
	p, err := lookPathFn("cc-select")
	if err != nil {
		return "", "", cliNone
	}
	out, err := probeCliVersion(p)
	if err != nil {
		return p, "", cliExternal
	}
	if v, ok := parseOurVersion(out); ok {
		return p, v, cliOurs
	}
	return p, "", cliExternal
}

// installEmbeddedCLI 把内置 CLI 安装到 PATH。返回 (结果, 冲突, 错误)：
// 冲突非 nil 时 HTTP 层转 409，错误转 500。
func installEmbeddedCLI(opts installOptions) (installResult, *installConflict, error) {
	path, ver, kind := detectInstalledCLI()

	if kind == cliExternal && !opts.ConfirmOverwrite {
		return installResult{}, &installConflict{
			Kind:    conflictExternal,
			Path:    path,
			Version: "unknown",
			Hint:    fmt.Sprintf("%s is not managed by CC Select; overwrite it explicitly to proceed", path),
		}, nil
	}

	// 已安装（本工具）→ 原地升级/重装；未安装 → 依序找首个可写候选目录。
	var targets []string
	if kind != cliNone {
		targets = []string{filepath.Dir(path)}
	} else {
		targets = opts.TargetDirs
	}

	fellBack := false
	for i, dir := range targets {
		dest := filepath.Join(dir, cliFileName())
		if err := opts.ExtractEmbedded(dest); err != nil {
			if os.IsPermission(err) {
				if i == len(targets)-1 {
					return installResult{}, &installConflict{
						Kind: conflictPermission,
						Path: dir,
						Hint: permissionHint(dir),
					}, nil
				}
				fellBack = true
				continue
			}
			return installResult{}, nil, fmt.Errorf("write %s: %w", dest, err)
		}

		res := installResult{InstalledPath: dest, InstalledVersion: opts.EmbeddedVersion}
		switch {
		case kind == cliNone:
			res.Action = "installed"
			if fellBack {
				res.Hint = fmt.Sprintf("installed to %s; add it to your PATH", dir)
			}
		case kind == cliOurs && ver == opts.EmbeddedVersion:
			res.Action = "reinstalled"
		case kind == cliOurs:
			res.Action = "upgraded"
		default: // cliExternal + confirmed
			res.Action = "overwritten"
		}
		if err := afterInstall(dir); err != nil {
			return installResult{}, nil, fmt.Errorf("post-install: %w", err)
		}
		return res, nil, nil
	}
	// targets 为空的理论兜底（调用方保证非空）。
	return installResult{}, &installConflict{Kind: conflictPermission, Hint: permissionHint("")}, nil
}

// pathCliDetector 是 cliDetector 的真实实现：包装 detectInstalledCLI。
// 外部/无法识别的二进制版本记为 "unknown"（前端据此展示覆盖确认，data-model §1.2）。
type pathCliDetector struct{}

func (pathCliDetector) Detect() (string, string, bool) {
	p, v, kind := detectInstalledCLI()
	switch kind {
	case cliNone:
		return "", "", false
	case cliOurs:
		return p, v, true
	default:
		return p, "unknown", true
	}
}

func permissionHint(dir string) string {
	if runtime.GOOS == "windows" {
		return "cannot write to " + dir + "; check directory permissions"
	}
	return fmt.Sprintf("no writable install directory (tried %s); fix permissions or set CC_SELECT_INSTALL_DIR", dir)
}
