package rcinteg

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cc-select/cc-select/internal/shell"
)

// unixStrategy 定位 zsh/bash 的 rc 文件（macOS / Linux / Windows Git Bash）。
//
// 跨平台：Git Bash on Windows 场景下，os.UserHomeDir() 返回 %USERPROFILE%，
// 恰为 Git Bash 的 $HOME，故 .bashrc 落点正确。因此本策略不按 OS 加 build 约束——
// 它是「zsh/bash 路径」维度，不是「Unix OS」维度（扩展点=shell 非 OS）。
type unixStrategy struct {
	shell shell.Shell
}

func (u unixStrategy) Resolve(home string) (string, error) {
	switch u.shell {
	case shell.Zsh:
		return filepath.Join(home, ".zshrc"), nil
	case shell.Bash:
		// 登录 bash（macOS Terminal/iTerm 默认开登录 shell）只读 .bash_profile，不读 .bashrc；
		// 交互非登录 bash（多数 Linux）读 .bashrc。故：若已存在 .bash_profile 就写它（覆盖登录
		// shell 场景；这类用户的 .bash_profile 通常也会 source .bashrc），否则写 .bashrc。
		profile := filepath.Join(home, ".bash_profile")
		if fileExists(profile) {
			return profile, nil
		}
		return filepath.Join(home, ".bashrc"), nil
	}
	return "", fmt.Errorf("不支持的 unix shell: %s", u.shell)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
