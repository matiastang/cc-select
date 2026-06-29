// Package app 聚合 cc-select 运行期依赖（config + secrets），供 CLI/Web 共用。
// 把依赖装配集中在一处，避免每个命令重复构造，也便于测试注入。
package app

import (
	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/secrets"
)

// App 持有命令运行所需的依赖。
type App struct {
	Config  *config.Config
	Secrets secrets.SecretStore
}

// New 装配默认依赖：从磁盘 Load 配置 + 系统 Keychain。
func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return &App{Config: cfg, Secrets: secrets.New()}, nil
}
