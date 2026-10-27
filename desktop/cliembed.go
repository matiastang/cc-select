package main

import (
	"embed"
	"fmt"
	"os"
)

// 内置 CLI（T017）：desktop/assets/cli/ 由 Makefile desktop-cli 目标在构建期拷入
// 与桌面端同 GOOS/GOARCH 的 cc-select 二进制，经 go:embed 嵌入 App。
// 全新检出时目录只有占位 README.md（go:embed 不匹配 . 开头文件，故占位不可
// 是 .gitignore）——embed 模式成立、包可构建；提取按文件名找不到二进制时
// 给出「先跑 make desktop-build」的明确错误。

//go:embed assets/cli
var embeddedCliFS embed.FS

// extractEmbeddedCli 把内置 CLI 写到 dest（0755）。
func extractEmbeddedCli(dest string) error {
	entries, err := embeddedCliFS.ReadDir("assets/cli")
	if err != nil {
		return fmt.Errorf("read embedded cli: %w", err)
	}
	name := cliFileName() // 精确匹配，避免混入文件被误装（评审 LOW）
	found := false
	for _, e := range entries {
		if e.Name() == name {
			found = true
			break
		}
	}
	if !found {
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
