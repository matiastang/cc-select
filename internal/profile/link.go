// link.go 是 Mode B「共享 ~/.claude 条目」的 OS 中立逻辑：幂等自愈地维护 profile 目录
// 里指向 ~/.claude 的链接。真正的链接创建（符号链接 / junction）由各平台的
// link_unix.go / link_windows.go 的 makeLink 实现。
package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// skipEntry 记录一个未能链接的条目及原因（供调用方告警，非致命）。
type skipEntry struct {
	Name   string
	Reason string
}

// shareEntries 把 claudeHome 中应共享的条目链接进 profileDir：
//  1. 白名单每个条目：目录型先在 claudeHome 预创建（确保 claude 写入落到共享位置），再 ensureLink；
//  2. claudeHome 中实际存在的其余条目（除 deny）→ ensureLink（尽力共享 extras）。
//
// deny 中的名字永不链接（如 settings.json）。返回被跳过的条目（非致命）。
func shareEntries(profileDir, claudeHome string, deny []string) ([]skipEntry, error) {
	denied := map[string]bool{}
	for _, d := range deny {
		denied[d] = true
	}

	linked := map[string]bool{} // 已处理，避免重复
	var skipped []skipEntry

	// 1. 白名单：预创建目录 + 链接。
	for _, e := range sharedEntries {
		if denied[e.name] {
			continue
		}
		target := filepath.Join(claudeHome, e.name)
		if e.targetRel != "" {
			// home 根 sibling（如 ~/.claude.json）：目标相对 claudeHome 解析。
			target = filepath.Join(claudeHome, e.targetRel)
		}
		if e.isDir {
			if err := os.MkdirAll(target, 0o700); err != nil {
				// 预创建失败不致命（可能 ~/.claude 只读），记录并继续。
				skipped = append(skipped, skipEntry{e.name, "预创建共享目录失败: " + err.Error()})
				continue
			}
		}
		if s, err := ensureLink(profileDir, e.name, target, e.isDir); err != nil {
			skipped = append(skipped, skipEntry{e.name, err.Error()})
		} else if s {
			skipped = append(skipped, skipEntry{e.name, "存在非空真实条目，已跳过"})
		}
		linked[e.name] = true
	}

	// 2. 实际存在的 extras（除 deny 与已处理）。
	entries, err := os.ReadDir(claudeHome)
	if err != nil {
		// claudeHome 不存在/不可读 = 没东西可共享（如未装 claude），非错误。
		return skipped, nil
	}
	for _, ent := range entries {
		name := ent.Name()
		if denied[name] || linked[name] {
			continue
		}
		target := filepath.Join(claudeHome, name)
		if s, lerr := ensureLink(profileDir, name, target, ent.IsDir()); lerr != nil {
			skipped = append(skipped, skipEntry{name, lerr.Error()})
		} else if s {
			skipped = append(skipped, skipEntry{name, "存在非空真实条目，已跳过"})
		}
		linked[name] = true
	}

	return skipped, nil
}

// ensureLink 幂等地让 profileDir/name 成为指向 target 的链接。
//
// lstat 分支：
//   - 不存在 → makeLink 创建；
//   - 是链接且指向 target → 空操作；
//   - 是链接但指向他处 → 删除重建（修陈旧）；
//   - 真实条目（非链接）→ 权威化：空的清掉重建；非空返回 skipped=true（警告，不毁数据）。
//
// 返回 (skipped, err)：skipped=true 表示因非空真实条目而未链接（非致命）。
func ensureLink(profileDir, name, target string, isDir bool) (skipped bool, err error) {
	link := filepath.Join(profileDir, name)
	_, statErr := os.Lstat(link)
	if os.IsNotExist(statErr) {
		return false, makeLink(target, link, isDir)
	}
	if statErr != nil {
		return false, statErr
	}

	// 已存在：链接（symlink 或 Windows junction）or 真实条目。
	// 用 Readlink 判定链接，而非 ModeSymlink：Windows junction（mklink /J，目录共享的主力）
	// 不设 ModeSymlink，但 Readlink 能读到其目标（Go 1.22+）。仅凭 ModeSymlink 会把已正确
	// junction 的目录误判成"非空真实条目"而告警跳过。
	if cur, rerr := os.Readlink(link); rerr == nil {
		if sameLink(cur, target) {
			return false, nil // 正确指向，空操作
		}
		// 陈旧链接 → 重建。
		if err := os.Remove(link); err != nil {
			return false, fmt.Errorf("移除陈旧链接 %s: %w", name, err)
		}
		return false, makeLink(target, link, isDir)
	}

	// Readlink 失败 → 真实条目（非链接）→ 权威化。
	if isEmptyReal(link) {
		if err := os.RemoveAll(link); err != nil {
			return false, fmt.Errorf("清空 %s: %w", name, err)
		}
		return false, makeLink(target, link, isDir)
	}
	// 非空真实条目：跳过，交由调用方告警。
	return true, nil
}

// sameLink 比较两个链接目标是否相同，容忍路径格式差异（分隔符、大小写）。
// filepath.Clean 统一分隔符；Windows 路径不区分大小写，额外用 EqualFold 兜底。
func sameLink(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if a == b {
		return true
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return false
}

// isEmptyReal 判断一个真实（非链接）路径是否为空（空文件或空目录）。
func isEmptyReal(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	if fi.IsDir() {
		entries, err := os.ReadDir(path)
		return err == nil && len(entries) == 0
	}
	return fi.Size() == 0
}

// pruneNonSettings 删除 profileDir 下除 settings.json 外的所有条目。
// Mode A（全隔离）用它保证目录只剩 settings.json，链接/陈旧实体不残留 → 真隔离。
func pruneNonSettings(profileDir string) error {
	entries, err := os.ReadDir(profileDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, ent := range entries {
		if ent.Name() == denySettings {
			continue
		}
		if err := os.RemoveAll(filepath.Join(profileDir, ent.Name())); err != nil {
			return fmt.Errorf("清理 %s: %w", ent.Name(), err)
		}
	}
	return nil
}
