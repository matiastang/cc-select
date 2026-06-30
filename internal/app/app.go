// Package app 聚合 cc-select 运行期依赖（config + prefs + secrets），供 CLI/Web 共用。
// 把依赖装配集中在一处，避免每个命令重复构造，也便于测试注入。
package app

import (
	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/cc-select/cc-select/internal/secrets"
)

// App 持有命令运行所需的依赖。
type App struct {
	Config  *config.Config
	Prefs   *prefs.Prefs
	Secrets secrets.SecretStore
}

// New 装配默认依赖：从磁盘 Load 配置 + 偏好 + 系统 Keychain。
//
// 偏好（prefs）损坏时不阻塞 CLI——降级为默认偏好（隔离模式按 ResolveMode 兜底）。
func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	pr, perr := prefs.Load()
	if perr != nil {
		pr = &prefs.Prefs{} // 偏好损坏不阻塞：用默认
	}
	return &App{Config: cfg, Prefs: pr, Secrets: secrets.New()}, nil
}
