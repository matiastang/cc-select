// Package profile 为每个 provider 维护一份独立的 claude 配置目录。
//
// 这是 cc-select 切换机制的核心（见 docs/engineering-decisions.md §6）：
// claude 优先读 ~/.claude/settings.json 的 env、覆盖 shell 环境变量，
// 故 cc-select 改用 CLAUDE_CONFIG_DIR（claude 官方支持）指向独立配置目录。
// 每个 provider 一个目录 ~/.cc-select/profiles/<id>/settings.json，
// ccs use X 只 export CLAUDE_CONFIG_DIR 指过去，claude 读那份 settings.json。
//
// 官方 provider（claude-official）不建 profile：切它 = unset CLAUDE_CONFIG_DIR，
// 让 claude 回默认 ~/.claude（复用用户既有登录态/全局配置）。
package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/prefs"
)

// ConfigVar 是切换时唯一要 export 的环境变量名（claude 官方支持）。
const ConfigVar = "CLAUDE_CONFIG_DIR"

// root 返回 profiles 根目录：与 providers.json 同级的 profiles/ 子目录。
// 这样 CC_SELECT_CONFIG 指向临时文件时，profiles 也落在 tempdir 下（测试友好）。
func root() (string, error) {
	cfgPath, err := config.ConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(cfgPath), "profiles"), nil
}

// Dir 返回某 provider 的 profile 目录绝对路径（~/.cc-select/profiles/<id>）。
// 先校验 id 合法（防路径穿越）——id 会拼进文件系统路径，含 ../、/ 等会逃逸 profiles 根目录。
func Dir(id string) (string, error) {
	if err := config.ValidateID(id); err != nil {
		return "", err
	}
	r, err := root()
	if err != nil {
		return "", err
	}
	return filepath.Join(r, id), nil
}

// Path 返回 settings.json 绝对路径 = Dir(id) + "/settings.json"。
func Path(id string) (string, error) {
	d, err := Dir(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "settings.json"), nil
}

// claudeSettings 是 claude settings.json 的最小子集：只需 env 字段。
type claudeSettings struct {
	Env map[string]string `json:"env"`
}

// Ensure 为 provider 写入（覆盖）其 profile 目录的 settings.json，内容为 {"env": env}。
// env 即该 provider 要给 claude 的 env（含明文 token）。目录 chmod 0700，文件 chmod 0600。
// 官方 provider（id == config.OfficialProviderID）：返回 ("", nil) 且不创建任何文件。
//
// 这是 Sync 的 ModeFull（全隔离）薄封装，保留给既有调用方/测试；新代码应直接用 Sync
// 以支持 Mode B（仅 settings.json 隔离）。需要写入完整 settings.json（含 permissions/hooks
// 等任意字段）时，用 EnsureRaw。
func Ensure(id string, env map[string]string) (string, error) {
	dir, _, err := Sync(id, env, prefs.ModeFull)
	return dir, err
}

// EnsureRaw 为 provider 写入（覆盖）其 profile 目录的 settings.json，内容为 data 原文。
// data 应是一段合法 JSON（调用方负责校验）——支持 env 之外的任意 settings.json 字段
// （permissions、hooks、model 等）。目录 chmod 0700，文件 chmod 0600，原子写。
// 官方 provider：返回 ("", nil) 且不创建任何文件。
func EnsureRaw(id string, data []byte) (dir string, err error) {
	if id == config.OfficialProviderID {
		return "", nil // 官方不建 profile
	}
	d, err := Dir(id)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return "", fmt.Errorf(i18n.T("errors.profile.createDir"), err)
	}
	// 统一以换行结尾（与历史行为一致，便于 diff）。
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(append([]byte{}, data...), '\n')
	}
	path := filepath.Join(d, "settings.json")
	// 原子写：临时文件（同目录）+ rename，保证 claude 不会读到半截文件。
	tmp, err := os.CreateTemp(d, ".settings-*.json.tmp")
	if err != nil {
		return "", fmt.Errorf(i18n.T("errors.profile.createTemp"), err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return "", fmt.Errorf(i18n.T("errors.profile.writeTemp"), err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		cleanup()
		return "", fmt.Errorf(i18n.T("errors.profile.chmodTemp"), err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", fmt.Errorf(i18n.T("errors.profile.closeTemp"), err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return "", fmt.Errorf(i18n.T("errors.profile.replace"), err)
	}
	return d, nil
}

// Remove 删除某 provider 的整个 profile 目录（幂等：不存在视为成功）。
// 官方 provider：无操作返回 nil。
func Remove(id string) error {
	if id == config.OfficialProviderID {
		return nil
	}
	d, err := Dir(id)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(d); err != nil {
		return fmt.Errorf(i18n.T("errors.profile.removeDir"), err)
	}
	return nil
}

// Exists 判断 profile 目录且 settings.json 存在。官方 provider 恒为 false。
func Exists(id string) (bool, error) {
	if id == config.OfficialProviderID {
		return false, nil
	}
	p, err := Path(id)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ReadEnv 读出某 profile settings.json 的 env（供 use 预览 / web 展示键名 / edit 合并旧值用）。
// 不存在或无 env 字段返回空 map + nil。
func ReadEnv(id string) (map[string]string, error) {
	p, err := Path(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf(i18n.T("errors.profile.read"), err)
	}
	var s claudeSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf(i18n.T("errors.profile.parse"), err)
	}
	if s.Env == nil {
		return map[string]string{}, nil
	}
	return s.Env, nil
}

// ReadRaw 读出某 profile settings.json 的完整原文（供 web 编辑时反映磁盘真实内容）。
// 这是"编辑反映真实配置"的关键：即便用户手改了文件，web 也读到当前真值。
// 文件不存在返回 (nil, nil)；官方 provider 无 profile，同样返回 (nil, nil)。
func ReadRaw(id string) ([]byte, error) {
	if id == config.OfficialProviderID {
		return nil, nil
	}
	p, err := Path(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf(i18n.T("errors.profile.read"), err)
	}
	return data, nil
}
