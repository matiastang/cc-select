# cc-select 桌面端（Wails v3）

设计契约见 `specs/003-desktop-client/`（spec / plan / contracts / tasks）。本文件只讲**怎么开发**。

## 前置

```sh
# wails3 CLI（与 go.mod 锁定版本一致）
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
```

Go 版本要求 1.26（wails v3 beta.34 要求，go.mod 已声明；GOTOOLCHAIN=auto 自动获取）。
macOS 需 Xcode CLT；Linux 需 `libgtk-3-dev libwebkit2gtk-4.1-dev`。

## 三种运行方式

| 方式 | 命令 | 适用 | 说明 |
|---|---|---|---|
| **全保真直跑** | `make desktop-build && ./bin/cc-select-desktop` | 验证完整行为（含 desktop 端点） | 前端资产嵌入，与发布形态一致 |
| **热重载 dev** | `wails3 task -dir desktop dev`（在仓库根执行） | 迭代 desktop/*.go 与前端 | 见下方已知边界 |
| **打包** | `make desktop-package` | 产出 .app / .exe / AppImage | 产物在 `desktop/build/bin/` |

## 热重载 dev 流的完整姿势

dev 模式下 WebView 加载 vite dev server，`/api` 经 vite 代理打到本地 `ccs gui`，因此需要两个终端：

```sh
# 终端 1：起 API 后端（vite 的 /api 代理目标，端口 7799）
./bin/cc-select gui --no-browser        # 或: go run . gui --no-browser

# 终端 2：起桌面 dev（构建 dev app + vite + 窗口）
wails3 task -dir desktop dev
```

**已知边界**：此模式下 `/api/v1/desktop/*`（桌面专属端点）不可达——代理目标是
`ccs gui`（web 形态，无这些端点），Desktop 设置区不会渲染。需要验证桌面端专属
功能（设置区/CLI 安装/更新桥）时，用「全保真直跑」方式。前端改动经 vite 热更
立即生效；`desktop/*.go` 改动触发自动重建。

## 构建/打包说明

- 打包链由仓库 **Makefile** 统一编排（`desktop-build`：前端 → CLI → 嵌入位 → go build；
  `desktop-package` 在其上加 wails3 打包）。wails3 Taskfile 中 frontend/bindings/tidy
  任务是 no-op（见 `build/Taskfile.yml` 头注释）——本仓库前端与 CLI 有自己的构建链。
- 版本注入：三平台 BUILD_FLAGS 都带 `-X .../internal/version.Version`，与 CLI 同源
  （根 Taskfile 的 `VERSION` 变量，git describe）。
- 修改 `build/config.yml` 的 `info`（产品名/identifier/版本）后，需同步手改
  `build/darwin/Info*.plist`、`build/linux/*`、`build/windows/*` 中的对应身份字段
  （脚手架的自动生成任务未引入）。
- macOS 产物为 adhoc 签名（首发策略 unsigned，见 spec A2）。

## 目录

```text
desktop/
├── main.go          # 应用装配（窗口/AssetServer/单实例）
├── bridge.go        # 桌面语义路由（update 拦截 + desktop 命名空间 + 透传）
├── info.go          # GET /api/v1/desktop/info
├── Taskfile.yml     # wails3 任务入口（build/package/dev）
├── build/           # 平台构建任务与资产（plist/desktop/nsis/图标）
└── assets/cli/      # 内置 CLI 二进制的 embed 位（构建期拷入，T017 接线）
```
