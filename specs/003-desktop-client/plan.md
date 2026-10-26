# Implementation Plan: Wails v3 桌面客户端（desktop-client）

**Branch**: `003-desktop-client` | **Date**: 2026-10-26 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/003-desktop-client/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

以 **Wails v3（锁 v3.0.0-beta.34）** 为壳、**完整复用现有资产**（React 前端 + `internal/*` Go 业务包 + 同一份 `~/.cc-select/` 数据）交付三平台桌面 App：新增顶层 `desktop/` 入口包，AssetServer 挂载现有 API handler（前端零改动）；更新流由桥接层拦截 `/api/v1/update*` 映射到 Wails Updater（GitHub provider + SHA256SUMS 校验 + 原子替换）；CLI 以 go:embed 内置并提供「安装到 PATH」；托盘/自启/单实例/通知全部采用 v3 内置服务。既有 CLI 与 `ccs gui` 代码路径不动，零回归由结构保证（见 Constitution Check）。

## Technical Context

**Language/Version**: Go 1.24（后端）；TypeScript 5.7 + React 18（前端，复用既有）

**Primary Dependencies**: `github.com/wailsapp/wails/v3 v3.0.0-beta.34`（**锁定**，含 updater/autostart/tray/notifications/single-instance 内置服务）；现有依赖不变（cobra、go-keyring 等）

**Storage**: 现有 `~/.cc-select/` 两层 JSON（providers.json + profiles/）与 prefs.json（扩展 `Desktop` 子结构，向后兼容）；不新增存储位置

**Testing**: Go 单测（desktop 桥接/安装器/偏好归一化，TDD 先行）+ vitest（前端 Desktop 设置区）+ 既有全量回归（Go 单测/集成/Playwright e2e）+ 三平台手工冒烟（quickstart.md）

**Target Platform**: macOS（arm64，M 系列）/ Windows（x64，WebView2）/ Linux（x64，AppImage）

**Project Type**: desktop-app（**附加构建目标**，与既有 CLI 单二进制并存，互不侵入）

**Performance Goals**: 冷启动 ≤ 2s（M 系列/主流 x64）；安装包 ≤ 40MB；常驻托盘内存 ≤ 150MB

**Constraints**: 既有功能零回归（FR-003）；R1 终端隔离红线（FR-009）；macOS 首发 unsigned；Wails 处于 Beta（桌面 API 已宣告稳定，锁版本升级）

**Scale/Scope**: 单用户本地工具；desktop 包预计 ≤ 1500 行 Go + 少量前端增量；CI 三平台 matrix

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| 原则 | 结论 | 说明 |
|---|---|---|
| I. Eval/Wrapper 拆分不可破坏 | ✅ 通过 | 桌面端不触碰 shell 集成：托盘切换 = 复制命令（research R5），无任何「绕过 wrapper 改环境」路径 |
| II. CLAUDE_CONFIG_DIR 隔离机制 | ✅ 通过 | 不改切换机制；Mode P 路由操作走既有 RoutesPanel/routes.json 语义；`current` 真值来源不变 |
| III. 跨平台三 OS 不可遗漏 | ✅ 通过 | 三平台为一等交付物；托盘/自启/单实例均用 v3 跨平台服务；Linux 无托盘 WM 有降级路径（spec Edge Case）；Windows 仅 PowerShell 的既有约束不变 |
| IV. 已定技术选型不漂移 | ✅ 通过（已完成变更流程） | 桌面 App 转正属重大选型变更：docs/requirements.md Q2 表、v0.1.0 决策记录、docs/tech-stack.md §0/§3 已于 2026-10-26 **先行更新**（先改文档再动代码）；Wails 依赖引入理由见 Complexity Tracking |
| V. 质量门禁与验收同步 | ✅ 通过 | `make check` 不受影响（desktop 纳入 vet/lint 范围）；行为变更同步 acceptance-tests.md（tasks 中含）；Playwright e2e 继续全量跑 |
| VI. 安全基线不降级 | ✅ 通过 | 不新增凭证写入路径：桌面端复用 config/secrets 既有读写（0600/0700、keychain 占位机制）；更新校验（SHA256SUMS）为新增的安全增强 |
| VII. 文档链真值与 spec 边界 | ✅ 通过 | 需求已录 docs/requirements.md v0.1.0（唯一入口）；本 spec 为特性工作区；验收后结论回流 docs/（tasks 末项） |
| VIII. 测试先行（TDD） | ✅ 通过 | research R8：桥接 handler/安装器/偏好归一化均先写失败测试（纯 Go 可测）；桌面壳薄层由冒烟脚本覆盖 |

**Phase 1 设计后复查**：无新增违例。唯一需持续盯防点——desktop 桥接层必须**只认 `/api/v1/` 前缀**且不吞 SPA fallback（research R1 已知坑），已写入 contracts/desktop-api.md 的错误契约。

## Project Structure

### Documentation (this feature)

```text
specs/003-desktop-client/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   ├── desktop-api.md   # 桌面模式 HTTP 端点契约（含 update 拦截语义）
│   └── release-assets.md# 发布产物命名/校验契约（updater 依赖）
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
desktop/                          # 新增：Wails v3 桌面入口（package main，独立构建目标）
├── main.go                       # application.New：窗口/AssetServer(Assets+Handler)/单实例/托盘/autostart/updater 装配
├── bridge.go                     # 桌面桥接层：拦截 /api/v1/update* → Wails Updater；desktop info/settings 端点
├── cliinstall.go                 # 内置 CLI 提取、版本探测、PATH 安装（幂等/权限回落）
├── tray.go                       # 托盘菜单装配（providers 列表=复制命令、打开切换面板、状态、退出）
├── updatebridge_test.go          # TDD：桥接语义（check/run 映射、错误契约、仅认 /api 前缀）
├── cliinstall_test.go            # TDD：安装器（干净装/覆盖/外来源确认/无权限回落，TempDir fake）
└── assets/                       # 图标/清单（tray icon、app icon、嵌入的 CLI 二进制由构建放入）

internal/web/
├── api.go                        # 修改：新增导出 Handler() http.Handler（约 3 行包装，无行为变化）
├── embed.go                      # 修改：导出 AssetsFS() 供 desktop AssetServer 复用

internal/prefs/
└── prefs.go                      # 修改：Prefs 增加 Desktop *DesktopPrefs 子结构 + 归一化

internal/frontend/src/
├── components/DesktopSettings*   # 新增：Desktop 设置区（capability 探测条件渲染 + 测试）
└── ...                           # 既有组件零改动（含 CheckUpdateButton）

Makefile                          # 新增 desktop-build/desktop-package 目标（不污染既有 build）
.github/workflows/                # release 流程扩展：三平台 matrix 产出桌面端并列上传
```

**Structure Decision**: 顶层 `desktop/` 单目录（research R1）——CLI（根 main.go）与桌面端（desktop/main.go）物理隔离、依赖单向（desktop → internal/*），既有代码仅两处 3 行级导出包装与一处向后兼容字段扩展。

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| 引入 wails/v3 依赖（技术约束「依赖从简」需给理由） | 用户已确认的桌面化需求（Issue #12 / v0.1.0）需要窗口/托盘/自启/自更新/打包体系，自研成本远超引入框架 | ① `webview/webview` 轻库：无打包/更新/托盘/自启，需求覆盖不到一半；② Tauri：需 Rust 壳 + Go sidecar 双进程（research 已比选）；③ 不引入：需求不成立 |
| 仓库出现第二个构建目标（desktop/） | 桌面 App 与 CLI 是不同交付物（.app/.exe/AppImage vs 单二进制），合并入口会让 CLI 二进制背上 Wails 依赖与体积 | build-tag 双入口同一 main 包：构建配置复杂、误构建风险高，且违背 Wails 工具链目录约定 |
