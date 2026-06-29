// Package web 提供 cc-select 的本地 Web 配置页（仅 127.0.0.1）。
//
// GUI 形态已定为"本地 Web 服务 + 浏览器"（见 docs/architecture.md §4）。
// cc-select gui 起一个 HTTP 服务，浏览器访问配置页，通过 REST API 读写
// 共享的 JSON 配置——与 CLI 走同一份 config.Save / secrets，保证一致。
package web

import (
	"embed"
	"io/fs"
)

//go:embed assets/*
var embeddedAssets embed.FS

// assetsFS 返回前端静态文件的 fs.FS（根指向 assets 目录）。
func assetsFS() fs.FS {
	sub, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		// //go:embed assets/* 保证了 assets 存在，Sub 不会失败。
		panic(err)
	}
	return sub
}
