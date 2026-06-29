package shell

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Detect 尝试推断当前 shell 类型，供 init 默认选择 emitter。
// 优先级：CC_SELECT_SHELL 环境变量 > SHELL 环境变量（Unix）> 平台默认。
// 检测失败返回 Unknown，由调用方处理（通常回退 zsh 或报错）。
func Detect() Shell {
	// 测试/显式覆盖入口。
	if v := Shell(os.Getenv("CC_SELECT_SHELL")); v != Unknown {
		if _, err := For(v); err == nil {
			return v
		}
	}

	shellPath := os.Getenv("SHELL")
	if shellPath != "" {
		base := filepath.Base(shellPath)
		switch {
		case strings.HasPrefix(base, "zsh"):
			return Zsh
		case strings.HasPrefix(base, "bash"):
			return Bash
		case strings.HasPrefix(base, "fish"):
			// fish 语法与 zsh/bash 不兼容，暂不支持；返回 Unknown 让调用方提示。
			return Unknown
		}
	}

	// Windows 无 SHEELL 变量，默认 PowerShell。
	if runtime.GOOS == "windows" {
		return PowerShell
	}

	return Zsh // Unix 默认 zsh
}
