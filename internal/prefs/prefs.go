// Package prefs 存取 cc-select 自身偏好（与 provider 配置无关的设置）。
//
// 目前只承载「隔离模式」（isolationMode）：决定 profile 目录是全隔离（Mode A）
// 还是仅 settings.json 隔离（Mode B，默认）。详见 docs/isolation-modes.md。
//
// 存储于 ~/.cc-select/prefs.json（与 providers.json 同目录），原子写。
// 仅全局默认值存这里；per-provider 覆盖存于 providers.json 的 Provider.IsolationMode，
// 由 prefs.ResolveMode 做三级优先级合并。
package prefs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Mode 是隔离模式。
type Mode string

const (
	// ModeSettingsOnly（Mode B，默认）：profile 目录仅 settings.json 隔离，
	// 其余条目链接回 ~/.claude 共享。
	ModeSettingsOnly Mode = "settings-only"
	// ModeFull（Mode A）：profile 目录整体隔离，只有 settings.json。
	ModeFull Mode = "full"
)

// DefaultMode 是未做任何设置时的兜底模式。
const DefaultMode Mode = ModeSettingsOnly

// Prefs 是 prefs.json 的完整内容。
type Prefs struct {
	// IsolationMode 是全局默认隔离模式；空串表示「未设置」，Resolve 时回退 DefaultMode。
	IsolationMode Mode `json:"isolationMode,omitempty"`
}

// Valid 判断一个模式值是否合法（空串合法，表示「未设置/继承」）。
func (m Mode) Valid() bool {
	switch m {
	case "", ModeSettingsOnly, ModeFull:
		return true
	}
	return false
}

// path 返回 prefs.json 绝对路径：与 providers.json 同目录。
// 复用 CC_SELECT_CONFIG 环境变量（指向 providers.json），prefs.json 落其同级——测试友好。
func path() (string, error) {
	if p := os.Getenv("CC_SELECT_CONFIG"); p != "" {
		return filepath.Join(filepath.Dir(p), "prefs.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cc-select", "prefs.json"), nil
}

// Load 读取偏好。文件不存在返回空 Prefs（各字段为零值，Resolve 时回退默认），不报错。
// 其他读取错误（权限、损坏 JSON）原样返回。
func Load() (*Prefs, error) {
	p, err := path()
	if err != nil {
		return nil, fmt.Errorf("定位 prefs: %w", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Prefs{}, nil
		}
		return nil, fmt.Errorf("读取 prefs: %w", err)
	}
	var pr Prefs
	if err := json.Unmarshal(data, &pr); err != nil {
		return nil, fmt.Errorf("解析 prefs（%s）: %w", p, err)
	}
	return &pr, nil
}

// Save 以原子方式写入偏好：先写同目录临时文件（0600），再 rename 覆盖。
func Save(pr *Prefs) error {
	p, err := path()
	if err != nil {
		return fmt.Errorf("定位 prefs: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("创建 prefs 目录: %w", err)
	}
	data, err := json.MarshalIndent(pr, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 prefs: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(p), ".prefs-*.json.tmp")
	if err != nil {
		return fmt.Errorf("创建临时文件: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("写入临时文件: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("设置权限: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("关闭临时文件: %w", err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		cleanup()
		return fmt.Errorf("替换 prefs: %w", err)
	}
	return nil
}
