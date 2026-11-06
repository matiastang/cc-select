# cc-select 前端（Web 配置页，桌面端同源）

本地 Web 配置页的 React 前端。构建产物会被 Go 二进制 `//go:embed` 打包（见 `internal/web/embed.go`）。同一份构建产物同时服务 Web（`cc-select gui`）与 Wails v3 桌面 App **双形态**——桌面端经 `desktop/` 桥接层复用 `internal/web` 的同一 handler（API + 静态资产），前端不分叉（specs/004 决策）。

## 开发

```bash
cd internal/frontend
npm install
npm run dev      # Vite dev server，/api 代理到本地 cc-select gui（端口 7799）
```

开发时需另起后端：`cc-select gui --no-browser --port 7799`。

## 构建（产出打进 Go 二进制）

```bash
cd internal/frontend
npm install
npm run build    # 产物输出到 ../web/assets/，被 internal/web embed
```

构建后 `go build .` 即可把最新前端打进二进制。

## 端到端测试（Playwright）

`e2e/` 下是 Playwright 套件，驱动真实浏览器访问由**真实 cc-select 二进制**（含 embed 的前端）serve 的配置页。
每个用例独占一个二进制进程 + 临时 `CC_SELECT_CONFIG`，互不污染、可并行。覆盖纯 JSON 添加、
完整 settings 落盘、编辑反映磁盘真值、官方 provider 按钮禁用、非法 JSON 前端拦截。

```bash
cd internal/frontend
npm ci
npx playwright install chromium   # 首次需下载浏览器
npm run test:e2e                  # 或在仓库根：make e2e
```

`playwright.config.ts` 的 `globalSetup` 会自动先构建前端 + 二进制，故无需手动 `make all`。
后端 CRUD 逻辑由 `internal/web/api_test.go` 覆盖；e2e 专测浏览器里的交互行为。

## 说明

- `vite.config.ts` 把 `outDir` 指向 `../web/assets`，`base: "./"` 保证相对路径。
- 未构建前端时，`internal/web/assets/index.html` 是占位 fallback（最小可用，纯 fetch 渲染）。
- 该前端已承载预设画廊、三步向导、连通性测试、限额查询等交互（v0.1.1 改版，specs/004）；设计系统为 shadcn/ui 模式（Radix + Tailwind），组件源码在 `src/components/ui/`。
