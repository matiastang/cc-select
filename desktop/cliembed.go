package main

import (
	"embed"
	"fmt"
	"os"
)

// 内置 CLI（T017）：desktop/assets/cli/ 由 Makefile desktop-cli 目标在构建期拷入
// 与桌面端同 GOOS/GOARCH 的 cc-select 二进制，经 go:embed 嵌入 App。
// 未跑 desktop-cli 时目录只有 .gitignore——embed 仍成立，提取时给出明确错误。

//go:embed assets/cli
var embeddedCliFS embed.FS

// extractEmbeddedCli 把内置 CLI 写到 dest（0755）。
func extractEmbeddedCli(dest string) error {
	entries, err := embeddedCliFS.ReadDir("assets/cli")
	if err != nil {
		return fmt.Errorf("read embedded cli: %w", err)
	}
	name := ""
	for _, e := range entries {
		if e.Type().IsRegular() && e.Name()[0] != '.' {
			name = e.Name()
			break
		}
	}
	if name == "" {
		return fmt.Errorf("embedded CLI missing: run `make desktop-build` (desktop-cli step) before building desktop app")
	}
	data, err := embeddedCliFS.ReadFile("assets/cli/" + name)
	if err != nil {
		return fmt.Errorf("read embedded cli %s: %w", name, err)
	}
	if err := os.WriteFile(dest, data, 0o755); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	return nil
}
