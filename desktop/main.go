// Command desktop 是 cc-select 的 Wails v3 桌面客户端入口。
//
// 与 CLI（仓库根 main.go）物理隔离：本包只做"壳"——窗口、资产服务、托盘、
// 更新与系统集成；业务逻辑全部复用 internal/* 包（同一份 ~/.cc-select 数据）。
// 设计与契约见 specs/003-desktop-client/（plan.md / contracts/）。
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// assetsPlaceholder 仅保证骨架阶段可编译运行；T009 起由 internal/web 的
// 真实前端资产（AssetsFS）接管，本占位随之移除。
//
//go:embed assets/index.html
var assetsPlaceholder embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "CC Select",
		Description: "cc-select 桌面客户端",
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assetsPlaceholder),
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "CC Select",
		Width:  1120,
		Height: 760,
		URL:    "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
