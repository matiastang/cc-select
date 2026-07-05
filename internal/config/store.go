package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cc-select/cc-select/internal/i18n"
)

// Load 读取配置。文件不存在时返回内置默认配置（含官方 provider），不报错。
// 其他读取错误（权限、损坏的 JSON）原样返回。
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, fmt.Errorf(i18n.T("errors.config.path"), err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Default(), nil
		}
		return nil, fmt.Errorf(i18n.T("errors.config.read"), err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf(i18n.T("errors.config.parse"), path, err)
	}
	if c.Providers == nil {
		c.Providers = map[string]Provider{}
	}
	return &c, nil
}

// Save 以原子方式写入配置：先写同目录临时文件（权限 0600），再 rename 覆盖。
// 同目录 rename 在 POSIX 上是原子的，避免多进程并发读写时读到半截文件。
func Save(c *Config) error {
	path, err := ConfigPath()
	if err != nil {
		return fmt.Errorf(i18n.T("errors.config.path"), err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf(i18n.T("errors.config.createDir"), err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf(i18n.T("errors.config.serialize"), err)
	}
	data = append(data, '\n')

	// 临时文件与目标同目录，保证 rename 原子（跨目录 rename 不保证原子）。
	tmp, err := os.CreateTemp(filepath.Dir(path), ".providers-*.json.tmp")
	if err != nil {
		return fmt.Errorf(i18n.T("errors.config.createTemp"), err)
	}
	tmpName := tmp.Name()
	// 失败路径清理：若写入或 rename 失败，移除残留临时文件。
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf(i18n.T("errors.config.writeTemp"), err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf(i18n.T("errors.config.chmodTemp"), err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf(i18n.T("errors.config.closeTemp"), err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf(i18n.T("errors.config.replace"), err)
	}
	return nil
}

// Provider 返回指定 ID 的 provider。不存在返回错误。
func (c *Config) Provider(id string) (Provider, error) {
	p, ok := c.Providers[id]
	if !ok {
		return Provider{}, fmt.Errorf(i18n.T("errors.provider.notFound"), id)
	}
	return p, nil
}
