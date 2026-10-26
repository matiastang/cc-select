# Research: Wails v3 桌面客户端（desktop-client）

**Date**: 2026-10-26 | **Status**: Complete（全部 NEEDS CLARIFICATION 已解决）

> 研究来源：Wails v3 官方文档（v3.wails.io，2026-10 抓取）、Wails GitHub Releases、本项目代码勘查（internal/web、internal/prefs、internal/updater、internal/frontend）。以下每项决策含 Decision / Rationale / Alternatives。

---

## R1. 桌面端代码落点与资产复用方式

**Decision**：新建顶层 `desktop/` 目录（`package main`）作为 Wails v3 应用入口；**不复制前端**——桌面端 AssetServer 直接复用 `internal/web` 的嵌入资产（`internal/web/assets`，经导出包装）与 API handler。

**Rationale**：
- Wails v3 `application.New(application.Options{ AssetServer: &assetserver.Options{ Assets: fs.FS, Handler: http.Handler } })` 支持静态资产 + 自定义 `http.Handler` 并存——静态资源走 Assets FS，未命中路径走 Handler。
- 现有前端全部使用相对路径 `fetch("/api/v1/...")`，挂上 Handler 后**前端零改动**即工作。
- `internal/web/api.go` 的 `newAPIHandler().routes()` 当前未导出——新增一个导出包装（约 3 行），`ccs gui` 与桌面端共用同一 handler，天然满足 FR-002/FR-015（单一数据、单一前端）。
- desktop 独立 package 保证 CLI 二进制（根 `main.go`）不引入 Wails 依赖，反之桌面二进制不影响 CLI——FR-003 零回归的结构保证。

**Alternatives**：
- *在根 main.go 加 build tag 区分 CLI/桌面两个入口*：被否——同一 main 包双形态增加构建复杂度与误构建风险，且 Wails 工具链（wails3 CLI/task）以独立应用目录为约定。
- *桌面端改用 Wails bindings 重写数据层*：被否——前端分叉（违反 FR-015），收益（类型安全绑定）对本项目不构成刚需，HTTP 契约已有测试覆盖。
- *桌面端内嵌完整 HTTP server 监听回环端口*：被否——AssetServer Handler 方案无需开端口，少一个本地攻击面，也少一个端口冲突处理分支。

**已知坑（官方文档/社区确认）**：自定义 Handler 与 SPA fallback 交互——未匹配静态资产的非 API 路径可能被喂给 Handler 或回落 index.html。缓解：桥接层显式只认 `/api/v1/` 前缀，其余返回 404 JSON，不吞 SPA fallback。

---

## R2. 更新流：桥接 Wails Updater，前端零改动（FR-006/FR-007 落地方式）

**Decision**：桌面模式下由 desktop 包的桥接 handler **拦截 `/api/v1/update/check` 与 `/api/v1/update`**，语义映射到 Wails v3 Updater 服务（headless 模式）；前端 `CheckUpdateButton` 及现有更新卡片 UX **完全不变**。CLI/Web 形态下这两个端点仍走 `internal/updater`（二进制自替换），行为零变化。

**Rationale**：
- Wails v3 Updater 提供：可插拔 provider（官方 GitHub provider）、校验（minisign 签名或 SHA256SUMS 摘要）、**原子替换 + 失败自动回滚 + 中断恢复**（helper 进程负责 swap）、默认更新窗口 UI（可换肤或 headless 自绘）。原子性直接满足 FR-007。
- **AppImage 明确支持原地热更**（经 `$APPIMAGE` 环境变量定位并替换自身）——Linux 首发格式的更新闭环成立。
- GitHub provider 按 GOOS/GOARCH 匹配 release 资产名，`ChecksumAsset` 指向 `SHA256SUMS` 即可完成摘要校验，无需先上 minisign 全签名体系（v0.1.0 从简，全签名留作增强）。
- 拦截而非改前端：FR-015 单一前端；且 web/desktop 两种形态的更新 UX 保持一致（SC-002 的 ≤3 次交互沿用现有卡片流）。

**产物格式约束（官方文档明确）**：macOS 更新包必须是 `.app` 打成的 **`.zip`**（不支持 dmg 热更）；Windows 为单 `.exe` 或其 zip（不支持 msi）；Linux 为 AppImage。发布产物命名契约见 contracts/release-assets.md。

**Alternatives**：
- *用 Updater 默认窗口 UI*：被否——与现有 web GUI 更新卡片 UX 割裂，且多引入一套 UI 主题适配。
- *前端检测桌面模式后调 Wails bindings*：被否——前端引入 wails runtime 依赖与模式分支，违反单一前端原则。
- *继续用 internal/updater 的 Replace 逻辑更新桌面 App*：被否——其实现是「同路径二进制替换」，对 .app bundle/AppImage 不成立，且无签名校验。

---

## R3. 内置 CLI 与「安装到 PATH」（FR-004/FR-005）

**Decision**：构建流水线先产出平台 CLI 二进制，`desktop/` 以 `go:embed` 嵌入与桌面端同 GOOS/GOARCH 的 `cc-select`；安装动作提取到平台约定目录（macOS/Linux：`/usr/local/bin`，无权限回落 `~/.bin` 并提示；Windows：`%LOCALAPPDATA%\Programs\cc-select\` + 写入用户 PATH）。安装前后做版本探测（`exec.LookPath` + `--version`）实现幂等与升级提示。

**Rationale**：
- go:embed 单文件零运行时依赖；平台匹配由构建矩阵保证（mac arm64 桌面嵌 mac arm64 CLI），无跨平台错配可能。
- 版本探测覆盖 spec 全部四个验收场景：干净安装 / 旧版覆盖 / 无权限指引 / 外来源不静默覆盖（探测到路径存在但版本/来源不明时弹确认）。
- Windows 写 HKCU PATH（无需管理员），macOS/Linux 回落策略避免权限死锁。

**Alternatives**：
- *App 内不含 CLI，引导去下载*：被否——用户已确认「内置 + 安装按钮」决策。
- *安装为符号链接/junction 指向 App 内文件*：被否——App 更新/移动会导致链接失效；复制独立文件最稳。

---

## R4. 桌面偏好存储（FR-008/FR-010 的开关项）

**Decision**：扩展 `internal/prefs.Prefs` 增加 `Desktop *DesktopPrefs` 可空子结构（trayResident / autoStart / autoUpdateCheck 三开关，默认值在加载侧归一）；沿用 prefs.json 原子写。

**Rationale**：
- `encoding/json` 对旧版本读新字段、新版本读旧文件都向后兼容（未知字段忽略、缺省字段为零值+归一化）——App 与 CLI 版本不一致的 Edge Case 在偏好层天然成立。
- 复用现有原子写与路径解析，不新增存储位置（与 FR-002 精神一致）。

**Alternatives**：*独立 desktop-prefs.json*：被否——两份偏好文件在 CLI/GUI/App 三端并发写时引入新的同步问题。

---

## R5. 托盘快捷切换语义定案（spec 假设 A5 → 本节闭合）

**Decision**：**v0.1.0 托盘不做「直接改状态」的切换**。托盘菜单提供：providers 列表（点击 = 复制 `ccs use <id>` 到剪贴板 + 系统通知确认）、「打开切换面板」（聚焦主窗口路由/切换区）、状态行、退出。Mode P 用户的按终端路由切换沿用主窗口既有 RoutesPanel；Mode A/B 用户粘贴复制的命令完成切换。

**Rationale**：
- R1 红线（宪法原则 I/II）下托盘**没有合法的「全局切换」语义**：cc-select 的激活态活在各 shell 的环境变量/路由表里，托盘无从安全地选定「哪个终端」。任何「托盘点了就切」的设计都会重蹈 cc-switch 全局切换的覆辙或制造误导。
- 复制命令方案零风险、零新语义，且与产品分工一致（R4：GUI 配置、命令行切换）；RoutesPanel 已覆盖 Mode P 的可视化切换。
- FR-009 的验收（列表 + 状态 + 快捷入口 + 不影响其他终端）全部满足。

**Alternatives**：
- *Mode P 下对「最近活跃 TID」执行 route switch*：被否——「最近活跃」判定引入新真值，跨终端串扰风险高，收益低。
- *托盘弹独立小窗口选终端再切换*：推迟——依赖 TID→终端的可读映射（如终端标题），留作后续增强（记入 spec 边界外）。

---

## R6. 单实例 / 自启 / 托盘 / 通知：全部用 Wails v3 内置能力

**Decision**：单实例（`application.Options` 的 SingleInstance 配置，二次启动回调里聚焦既有窗口）、开机自启（v3 内置 Autostart 服务 `app.Autostart`，按平台走原生机制：macOS 登录项 / Windows 注册表 / Linux XDG autostart）、托盘（v3 tray API，含菜单与窗口吸附）、通知（v3 notifications 服务，WinRT toast）。

**Rationale**：四项均为 v3 官方文档列明的一等能力，自研任何一项都是纯成本；A6 假设中「实现技术 plan 定案」由此闭合为「用内置服务」。

**Alternatives**：*自研 loginitems/plist/registry/desktop 文件*：被否——三平台 × 三项的维护面，无差异化收益。

---

## R7. 构建与发布（FR-013 / SC-004）

**Decision**：
- 版本锁定 `github.com/wailsapp/wails/v3 v3.0.0-beta.34`（go.mod require），升级走显式变更。
- Makefile 新增 `desktop-*` 目标（build/package/clean），不污染现有 `build`；`wails3` CLI 仅在打包阶段需要。
- 发布：GitHub Actions matrix 三平台原生 runner（macos-arm64 / windows / ubuntu）；产物按 contracts/release-assets.md 命名（含 SHA256SUMS）；与现有 CLI release 流程同一条 tag 触发，产物并列上传。
- macOS 首发 unsigned：产物不公证，README/Release notes 写「右键 → 打开」指引（A2）；Windows 无证书，接受 SmartScreen 首次提示并文档说明。

**Rationale**：桌面构建需要各平台原生工具链（mac 的 WKWebView 绑定等），matrix 是唯一稳妥路径；同 tag 双产物（CLI + 桌面）保证版本号对齐，避免版本漂移。

**Alternatives**：*交叉编译出三平台*：被否——Wails 桌面构建依赖各平台原生 webview 工具链，社区结论是原生 runner 构建。

---

## R8. 开发与测试策略（宪法 V/VIII 落地）

**Decision**：
- **Go 单测先行（TDD）**：desktop 桥接 handler（update 拦截、desktop info/settings 端点）、CLI 安装器（版本探测/幂等/权限回落，文件系统用 t.TempDir fake）、DesktopPrefs 归一化——全部可纯 Go 测试，不依赖 Wails 运行时。
- **前端**：Desktop 设置区条件渲染组件走 vitest（既有体系）；`capability` 探测逻辑单测。
- **回归**：现有 Go 单测/集成/Playwright e2e 全量跑（web 形态零改动即应全绿）。
- **桌面冒烟**：quickstart.md 给三平台手工脚本（挂 FR/SC 映射），CI 阶段先 mac runner 上 `wails3 build` 冒烟 + 产物存在性校验，GUI 自动化 e2e 不进 v0.1.0（Wails v3 驱动自动化尚不成熟，记为已知边界）。

**Rationale**：桌面壳（窗口/托盘/autostart）薄且被 Wails API 覆盖，把可测逻辑全部下沉到纯 Go 层是最大化 TDD 覆盖面的结构选择。

---

## 参考链接

- Updater：https://v3.wails.io/guides/updater （provider/签名/原子替换/AppImage 热更）
- Autostart：https://v3.wails.io/features/autostart/basics
- Single Instance：https://v3.wails.io/guides/single-instance
- 系统托盘：https://v3.wails.io/features/menus/systray
- 通知：https://v3.wails.io/features/notifications/overview
- Asset Server（Handler/Middleware）：https://v3.wails.io/contributing/asset-server
- What's New in v3：https://v3.wails.io/whats-new
- Wails Releases（v3.0.0-beta.34）：https://github.com/wailsapp/wails/releases
