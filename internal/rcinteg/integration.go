// Package rcinteg 负责「shell 集成一键安装」的全部领域逻辑：
// 生成带 marker 的 ccs() 代码块、定位各 shell 的 rc 文件、幂等地写入/替换/备份。
//
// 设计要点（见 docs/distribution.md §2）：
//   - 扩展点是 shell 类型而非 OS：加 shell 只加一个 Strategy，不改控制流。
//   - marker 块自描述：rc 里的集成段带固定 begin/end 标记，检测/幂等/升级/卸载共用。
//   - 委托而非硬算：PowerShell 的 $PROFILE 让 pwsh/powershell 自己报，不维护脆弱路径表。
//   - 引擎与 IO 解耦：composeManaged 只操作「路径+snippet」字符串，易测。
//
// 依赖方向：rcinteg → shell（渲染）。不依赖 web/cli，可被 CLI init、Web API、未来
// cc-select install 命令共同复用，杜绝多份实现漂移。
package rcinteg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cc-select/cc-select/internal/shell"
)

// rc 文件中受管理代码块的边界标记。模板（init_zsh.tmpl / init_powershell.tmpl）
// 渲染出的 snippet 自带这两行；此处常量用于在用户 rc 中定位/替换整块。
// 模板里的标记行必须以此处子串开头，由 TestRenderInit_ZshContainsMarker /
// TestRenderInit_PowerShellSnippet 锁定一致性。
const (
	markerBegin = "# >>> cc-select shell integration"
	markerEnd   = "# <<< cc-select shell integration <<<"
)

// 安装动作枚举。
const (
	ActionAppended = "appended" // 首次写入
	ActionUpdated  = "updated"  // marker 块已存在，内容更新
	ActionNoop     = "noop"     // marker 块已存在且内容未变
	ActionManual   = "manual"   // 无法自动写，返回 snippet 给用户手动处理
)

// Strategy 描述某 shell 的 rc 文件定位策略。扩展点：新增 shell 实现此接口即可。
type Strategy interface {
	// Resolve 返回目标 rc 文件绝对路径。home 来自 os.UserHomeDir()（测试可注入）。
	// 无法自动定位（如 PowerShell 未安装）时返回 error，调用方据此降级为手动指引。
	Resolve(home string) (rcPath string, err error)
}

// Status 是 GET /shell-integration 返回的当前安装状态。
type Status struct {
	Supported      bool   // 该 shell 是否被 cc-select 支持（fish=false）
	Shell          string // Detect() 结果
	Installed      bool   // rc 中已有 marker 块
	Legacy         bool   // rc 中有老的无 marker ccs()（可提示升级）
	RCPath         string // 解析出的目标 rc；空=无法自动写
	CanAutoInstall bool   // false → 前端只给手动指引
}

// InstallResult 是 POST /shell-integration/install 的结果。
type InstallResult struct {
	Action  string // appended | updated | noop | manual
	Shell   string
	RCPath  string
	Snippet string // action=manual 时给用户粘贴（不支持 shell 时为空）
	Message string
}

// resolveShell 把用户传入的 shell 名归一化：去空白 + 转小写，接受 PowerShell/Zsh/BASH
// 等自然写法；空串/未知则回退 shell.Detect()。避免大小写导致 shell.For 拒绝。
func resolveShell(name string) shell.Shell {
	s := shell.Shell(strings.ToLower(strings.TrimSpace(name)))
	if s == shell.Unknown {
		return shell.Detect()
	}
	return s
}

// RenderInit 生成带 marker 的 ccs() 集成代码块。shellName 为空或自然大小写均可。
// 返回 snippet、实际使用的 shell、错误。CLI init 与 Web 安装共用此函数，杜绝漂移。
func RenderInit(shellName string) (snippet string, s shell.Shell, err error) {
	bin, err := os.Executable()
	if err != nil {
		return "", shell.Unknown, fmt.Errorf("定位 cc-select 可执行文件: %w", err)
	}
	// 解析符号链接拿真实路径；EvalSymlinks 出错返回空串，必须用临时变量承接，不能直接覆盖 bin。
	if resolved, err := filepath.EvalSymlinks(bin); err == nil {
		bin = resolved
	}
	s = resolveShell(shellName)
	emitter, err := shell.For(s)
	if err != nil {
		return "", s, err
	}
	return emitter.InitSnippet(bin), s, nil
}

// DetectStatus 探测当前 shell 的集成状态（供 GET）。
func DetectStatus() Status {
	s := shell.Detect()
	st := Status{Shell: string(s)}
	strat, ok := strategyFor(s)
	if !ok {
		// fish 等不支持。
		st.Supported = false
		return st
	}
	st.Supported = true
	home, err := os.UserHomeDir()
	if err != nil {
		st.CanAutoInstall = false
		return st
	}
	rc, err := strat.Resolve(home)
	if err != nil || rc == "" {
		// PowerShell 探测失败等 → 手动降级。
		st.CanAutoInstall = false
		return st
	}
	st.RCPath = rc
	st.CanAutoInstall = true
	content, _ := os.ReadFile(rc)
	c := string(content)
	if strings.Contains(c, markerBegin) {
		st.Installed = true
	} else if hasLegacy(c, s) {
		st.Legacy = true
	}
	return st
}

// Install 把集成写入当前 shell 的 rc（供 POST）。
// 不支持的 shell（fish 等）返回 manual 结果（无 snippet + 提示），而非 error。
func Install(shellName string) (InstallResult, error) {
	s := resolveShell(shellName)
	strat, ok := strategyFor(s)
	if !ok {
		return InstallResult{
			Action:  ActionManual,
			Shell:   string(s),
			Message: fmt.Sprintf("%s 暂不支持一键安装，请使用 zsh / bash / PowerShell", s),
		}, nil
	}
	snippet, _, err := RenderInit(string(s)) // 已是受支持 shell，RenderInit 不会因 For 报错
	if err != nil {
		return InstallResult{Shell: string(s)}, err
	}
	return installWith(s, snippet, strat)
}

// installWith 是 Install 的可注入核心（测试可传 mock strategy）：
// 定位 rc → 写入；Resolve 失败则降级为 manual。
func installWith(s shell.Shell, snippet string, strat Strategy) (InstallResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return manualResult(s, snippet, "无法定位用户目录，请手动添加以下内容"), nil
	}
	rc, err := strat.Resolve(home)
	if err != nil || rc == "" {
		return manualResult(s, snippet, manualMessage(s, err)), nil
	}
	action, err := writeManagedBlock(rc, snippet)
	if err != nil {
		return InstallResult{Shell: string(s)}, fmt.Errorf("写入 %s: %w", rc, err)
	}
	return InstallResult{
		Action:  action,
		Shell:   string(s),
		RCPath:  rc,
		Message: successMessage(s, rc, action),
	}, nil
}

// manualResult 构造降级结果：返回 snippet 让用户手动处理。
func manualResult(s shell.Shell, snippet, msg string) InstallResult {
	return InstallResult{Action: ActionManual, Shell: string(s), Snippet: snippet, Message: msg}
}

// writeManagedBlock 把 snippet 作为 marker 块写入 rcPath（幂等），返回动作类型。
//   - rc 无 marker 块 → 备份后追加（appended）
//   - rc 有 marker 块 → 整块替换（updated）；内容未变则 noop
// 任何实际写入（非 noop）前，若原 rc 非空则备份（首次备份不覆盖，保留最早原始态）。
func writeManagedBlock(rcPath, snippet string) (action string, err error) {
	originalBytes, rerr := os.ReadFile(rcPath)
	original := ""
	if rerr == nil {
		original = string(originalBytes)
	} else if !os.IsNotExist(rerr) {
		return "", rerr
	}

	hadMarker := strings.Contains(original, markerBegin)
	composed := composeManaged(original, snippet)
	if composed == original {
		return ActionNoop, nil
	}

	// 实际写入前备份原 rc（已存在备份则不覆盖，保留最早的原始态；updated 场景同样保护）。
	if original != "" {
		backup := rcPath + ".cc-select.bak"
		if _, statErr := os.Stat(backup); os.IsNotExist(statErr) {
			perm := os.FileMode(0o600)
			if fi, err := os.Stat(rcPath); err == nil {
				perm = fi.Mode().Perm() // 备份沿用原 rc 权限，避免意外改变权限位
			}
			_ = os.WriteFile(backup, []byte(original), perm)
		}
	}

	if err := atomicWriteRC(rcPath, []byte(composed)); err != nil {
		return "", err
	}
	if hadMarker {
		return ActionUpdated, nil
	}
	return ActionAppended, nil
}

// composeManaged 把 snippet 作为 marker 块合并进 original rc 全文：
//   - 逐行扫描，删除【所有】既有的 marker 块（begin 行到 end 行，含两端）——
//     处理多次安装/孤儿块/升级，保证最终只留下一个新的受管块；
//   - 再在末尾追加新 snippet（前置空行分隔）。
//
// 行前缀匹配（仅认真正的 marker 行），避免用户注释里偶然包含 marker 子串时
// 误判；不会「截断到 EOF」——若只有孤儿 begin 而无 end，其后内容仍会被视作
// 块内而一并移除（那本就是损坏的受管块），但不会波及 begin 之前的内容。
// 纯函数，不碰 IO，便于单测。
func composeManaged(original, snippet string) string {
	snippet = strings.TrimRight(snippet, "\n") + "\n"

	var kept []string
	inBlock := false
	for _, ln := range strings.Split(original, "\n") {
		if inBlock {
			if strings.HasPrefix(strings.TrimSpace(ln), markerEnd) {
				inBlock = false
			}
			continue // 块内行（含 end 行）一律丢弃
		}
		if strings.HasPrefix(strings.TrimSpace(ln), markerBegin) {
			inBlock = true
			continue
		}
		kept = append(kept, ln)
	}

	base := strings.TrimRight(strings.Join(kept, "\n"), "\n")
	if base == "" {
		return snippet
	}
	return base + "\n\n" + snippet
}

// atomicWriteRC 以临时文件+rename 原子写回 rc 全文，保留原文件权限。
// 若 rc 是符号链接（dotfiles 仓库常见），写到链接指向的真实文件，保留软链
// （回归等价于 shell 的 `>>` 跟随软链行为；避免 os.Rename 把软链替换成普通文件）。
func atomicWriteRC(path string, data []byte) error {
	writePath := path
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			writePath = resolved
		}
	}
	dir := filepath.Dir(writePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("创建目录: %w", err)
	}
	perm := os.FileMode(0o644)
	if fi, err := os.Stat(writePath); err == nil {
		perm = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(dir, ".cc-select-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, writePath); err != nil {
		cleanup()
		return err
	}
	return nil
}

// hasLegacy 检测老的无 marker 集成块（各 shell 函数定义语法不同）。best-effort。
func hasLegacy(content string, s shell.Shell) bool {
	switch s {
	case shell.Zsh, shell.Bash:
		return strings.Contains(content, "ccs()")
	case shell.PowerShell:
		return strings.Contains(content, "function ccs")
	}
	return false
}

// strategyFor 返回 shell 对应的 rc 定位策略。新增 shell 在此注册。
func strategyFor(s shell.Shell) (Strategy, bool) {
	switch s {
	case shell.Zsh, shell.Bash:
		return unixStrategy{shell: s}, true
	case shell.PowerShell:
		return pwshStrategy{}, true
	}
	return nil, false
}

func successMessage(s shell.Shell, rc string, action string) string {
	switch action {
	case ActionNoop:
		return fmt.Sprintf("%s 集成已是最新（%s）", s, rc)
	case ActionUpdated:
		return fmt.Sprintf("已更新 %s 集成（%s）；新开终端使最新版本生效", s, rc)
	}
	switch s {
	case shell.PowerShell:
		return fmt.Sprintf("已写入 %s。请重启 PowerShell 或执行 . $PROFILE 使 ccs 生效", rc)
	default:
		return fmt.Sprintf("已写入 %s。请新开终端或执行 source %s 使 ccs 生效", rc, rc)
	}
}

func manualMessage(s shell.Shell, resolveErr error) string {
	if s == shell.PowerShell {
		return "未能自动定位 $PROFILE（可能未安装 PowerShell）。请把以下内容加入你的 PowerShell $PROFILE 后执行 . $PROFILE"
	}
	if resolveErr != nil {
		return fmt.Sprintf("未能定位 rc 文件（%v）。请手动添加以下内容", resolveErr)
	}
	return "请手动添加以下内容到对应启动脚本"
}
