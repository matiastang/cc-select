package updater

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cc-select/cc-select/internal/i18n"
)

// Manager 标识二进制的安装来源（包管理器）。
type Manager int

const (
	ManagerNone Manager = iota
	ManagerHomebrew
	ManagerScoop
)

func (m Manager) String() string {
	switch m {
	case ManagerHomebrew:
		return KindHomebrew
	case ManagerScoop:
		return KindScoop
	}
	return "none"
}

// Context 是安装环境快照，每次 Run 构建一次。
type Context struct {
	ExecPath string  // os.Executable() 解析符号链接后的真实路径
	IsDev    bool    // version.Version == "dev"
	Manager  Manager // 安装来源（brew/scoop/none）
	Writable bool    // 安装目录对当前用户可写
}

// executableFn 可注入：测试返回 temp 目录里的 stub 路径。
var executableFn = os.Executable

// detectContextFn 是 Run 使用的上下文构造函数，测试可整体替换（= rcinteg 可注入核心模式）。
var detectContextFn = DetectContext

// DetectContext 从 os.Executable + version.Version 构建 Context。
// 符号链接必须解析：Homebrew 把 <prefix>/bin/cc-select 软链到 Cellar，
// 不解析则 DetectManager 看不到 Cellar 路径（= rcinteg.RenderInit 的同款写法）。
func DetectContext() (Context, error) {
	bin, err := executableFn()
	if err != nil {
		return Context{}, i18n.Ew("errors.update.locateExe", err)
	}
	if resolved, err := filepath.EvalSymlinks(bin); err == nil {
		bin = resolved
	}
	return Context{
		ExecPath: bin,
		IsDev:    IsDevBuild(),
		Manager:  DetectManager(bin),
		Writable: IsWritable(filepath.Dir(bin)),
	}, nil
}

// DetectManager 按路径分类安装来源。纯函数，可单测。
// 归一化为小写 + 正斜杠后做子串匹配；反斜杠替换用 strings.ReplaceAll 而非
// filepath.ToSlash——后者 OS 相关（Unix 上 \ 是合法文件名字符，不会转换），
// 而本函数是纯分类器，输入可能是任一平台风格的路径（测试即跨平台跑 Windows 路径）。
//   - Homebrew：/opt/homebrew/Cellar/... 与 /usr/local/Cellar/... 都含 "/cellar/"
//   - Scoop：%USERPROFILE%\scoop\apps\cc-select\current\... 含 "/scoop/apps/"
func DetectManager(execPath string) Manager {
	p := strings.ToLower(strings.ReplaceAll(execPath, "\\", "/"))
	if strings.Contains(p, "/cellar/") {
		return ManagerHomebrew
	}
	if strings.Contains(p, "/scoop/apps/") {
		return ManagerScoop
	}
	return ManagerNone
}

// IsWritable 报告目录对当前用户是否可写。best-effort：创建并删除一个探针文件。
func IsWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".cc-select-wprobe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}
