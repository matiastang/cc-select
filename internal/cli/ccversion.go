package cli

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/cc-select/cc-select/internal/i18n"
)

// ccversion.go 是 best-effort 的 Claude Code 版本探测（specs/002 research D6 / FR-007）。
//
// 用途：modelPicker 需 CC ≥ 2.1.242；旧版本注入无害（官方 schema 对未知键
// additionalProperties:true，直接忽略），但用户看不到效果——此时给一行升级提示。
// 任何探测失败（无 claude、超时、输出怪异）都静默跳过：绝不阻断 use 主流程。

// pickerMinVersion 是 modelPicker 支持的最低 Claude Code 版本。
var pickerMinVersion = [3]int{2, 1, 242}

var semverRe = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

// supportsModelPicker 从 `claude --version` 输出判断是否支持 modelPicker。
// 解析失败返回 false（保守：宁可误提示，不可漏提示）。
func supportsModelPicker(versionOut string) bool {
	m := semverRe.FindStringSubmatch(versionOut)
	if m == nil {
		return false
	}
	v := [3]int{}
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return false
		}
		v[i] = n
	}
	for i := 0; i < 3; i++ {
		if v[i] != pickerMinVersion[i] {
			return v[i] > pickerMinVersion[i]
		}
	}
	return true // 恰好等于门槛版本
}

// warnIfPickerUnsupported 在 use 成功后调用：版本可探测且低于门槛时返回提示文案，
// 否则返回空串。探测失败（无 claude/超时）返回空串（静默）。
func warnIfPickerUnsupported() string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "claude", "--version").Output()
	if err != nil {
		return ""
	}
	if supportsModelPicker(string(out)) {
		return ""
	}
	return i18n.T("cli.use.pickerUpgradeWarning")
}
