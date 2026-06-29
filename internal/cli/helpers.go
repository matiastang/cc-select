package cli

import (
	"os"

	"github.com/cc-select/cc-select/internal/config"
)

// envOrDefault 读环境变量，未设置返回空串。
func envOrDefault(name string) string {
	return os.Getenv(name)
}

// appLoadConfig 仅加载配置（不构造 secrets），用于只读命令补全展示信息。
func appLoadConfig() (*config.Config, error) {
	return config.Load()
}
