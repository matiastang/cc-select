package web

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/cc-select/cc-select/internal/i18n"
)

// OpenURL 尝试用系统默认浏览器打开 url。失败不报错（仅记录由调用方处理）。
// 不引第三方库，按平台调用 open/xdg-open/cmd。
func OpenURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		// "start" 是 cmd 内建，需经 cmd /c。
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default: // linux/*bsd
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf(i18n.T("errors.web.openBrowser"), err)
	}
	// 不等待浏览器进程（gui 命令本身会阻塞在 HTTP 服务上）。
	return nil
}
