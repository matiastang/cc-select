package rcinteg

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cc-select/cc-select/internal/i18n"
)

// pwshStrategy 定位 PowerShell 的 $PROFILE。
//
// 不硬算路径表（PS5/PS7、OneDrive 重定向 Documents、跨平台 PS Core 路径全不同且易错），
// 而是委托 PowerShell 自身报 $PROFILE——它会返回重定向后的真实绝对路径，零维护。
//
// $PROFILE 在进程生命周期内不变，故缓存首次探测结果：DetectStatus（每次 GET 都调）
// 与 Install（POST）以及未来调用方共享同一结果，避免每次开配置页都 spawn
// pwsh（~200ms-1s，最坏被 EDR/组策略拖到 5s）。
type pwshStrategy struct{}

// pwshMemo 是 $PROFILE 探测的进程级缓存。done 标记是否已探测，避免重复 spawn。
// 用 mutex（而非 sync.Once）以便 resetPwshCache 在测试间清空。
type pwshMemo struct {
	mu   sync.Mutex
	done bool
	path string
	err  error
}

var pwshCache pwshMemo

// profileProbe 探测 $PROFILE；声明为变量以便测试注入（模拟 pwsh 存在/缺失/返回路径），
// 避免单测依赖真实 PowerShell 进程。生产代码用 detectPwshProfile。
var profileProbe = detectPwshProfile

// resetPwshCache 清空 $PROFILE 缓存，仅用于测试隔离（每个用例独立探测）。
func resetPwshCache() {
	pwshCache.mu.Lock()
	defer pwshCache.mu.Unlock()
	pwshCache.done = false
	pwshCache.path = ""
	pwshCache.err = nil
}

// Resolve 忽略 home：$PROFILE 由 pwsh 返回绝对路径，与 os.UserHomeDir() 无关。
func (pwshStrategy) Resolve(home string) (string, error) {
	pwshCache.mu.Lock()
	defer pwshCache.mu.Unlock()
	if !pwshCache.done {
		pwshCache.path, pwshCache.err = profileProbe()
		pwshCache.done = true
	}
	return pwshCache.path, pwshCache.err
}

func detectPwshProfile() (string, error) {
	if p, err := queryProfile("pwsh"); err == nil { // PS7 (pwsh)，跨平台
		return p, nil
	}
	if runtime.GOOS == "windows" {
		if p, err := queryProfile("powershell.exe"); err == nil { // PS5.1，仅 Windows
			return p, nil
		}
	}
	return "", i18n.E("errors.rcinteg.powershellNotFound")
}

// queryProfile 跑 <exe> -NoProfile -Command '$PROFILE' 取 profile 绝对路径。
//   - -NoProfile 必加：避免启动时加载用户 profile（递归/慢/副作用）。
//   - 超时 5s：个别环境首次启动慢或卡住时不阻塞 Web 请求。
func queryProfile(exe string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, exe, "-NoProfile", "-Command", "$PROFILE").Output()
	if err != nil {
		return "", err
	}
	p := strings.TrimSpace(string(out))
	if p == "" {
		return "", fmt.Errorf(i18n.T("errors.rcinteg.emptyProfile"), exe)
	}
	return p, nil
}
