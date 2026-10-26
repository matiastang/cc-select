# Quickstart: UI/UX 改版与快速上手（v0.1.1）

**Date**: 2026-10-26 | **Spec**: [spec.md](./spec.md)

## 1. 环境准备

```bash
# 前端依赖（含本期新增 tailwind/radix/lucide）
cd internal/frontend && npm install

# Go 侧零新依赖；GoReleaser/lefthook 既有
make check        # 应全绿（含新增 presets-lint）
```

## 2. 开发运行

```bash
# 方式一：Web 形态（热更）
cd internal/frontend && npm run dev          # vite :9245 起，/api 代理 :7799
make dev && ./bin/cc-select gui              # 另一终端起后端（自动开浏览器则关掉，用 9245）

# 方式二：桌面形态（wails dev；/desktop/* 端点可达）
wails3 task -dir desktop dev                 # 仓库根执行；仍需 ccs gui 在 7799（已知边界，desktop/README）

# 方式三：整包构建验证
make all                                     # CLI + 前端 embed
make desktop-build                           # 桌面（需 desktop-cli 前置，见 Makefile）
```

## 3. 冒烟路径（对应 US）

1. **US1 导航**：开 GUI → 侧边栏六模块（Web 形态无「桌面端」）→ 窄拉窗口 <880px → 顶部 tab。
2. **US2 设计系统**：切 light/dark/system → 各模块无破色；删光 provider → 空状态引导。
3. **US3 向导**：添加服务商 → 画廊选 MiniMax → 粘 key → 保存（不展开高级）→ `ccs use <id>` 可切换。
4. **US5 远端预设**：`curl -X POST localhost:7799/api/v1/presets/refresh` → 画廊出现远端 meta；断网重启 → builtin 兜底提示。
5. **US6 连通性**：向导 Step1「测试连接」→ 错 key 显示 auth 错误分类。
6. **US7 用量**：GLM provider 卡片 → 刷新用量 → 5h 窗口百分比；MiniMax → 显示不支持。

## 4. 三平台桌面人工验收清单（发布门禁，每平台执行）

- [ ] 六模块导航 + 深浅色切换 + 向导全流程（含 probe/用量）
- [ ] macOS：WKWebView（.app bundle 与裸二进制两形态——后者通知降级为既有语义）
- [ ] Windows：WebView2（PowerShell 环境；SAC 开启态按 docs/windows-support §7 记录）
- [ ] Linux：webkit2gtk（AppImage 基线发行版，如 Ubuntu 22.04）
- [ ] 托盘/自启/更新桥回归（specs/003 AC18 既有用例不回退）

## 5. 常见问题

- **画廊只有内置预设**：看 `presets-source-badge`；离线/被禁用/校验失败均回落 builtin，属预期（FR-013/014）。
- **probe 报 network 但浏览器可访问**：GUI 后端发起探测（非浏览器），检查系统代理/防火墙对 Go 进程的限制。
- **用量显示「不支持」**：该服务商无公开限额端点且响应头无限流信息（FR-020 预期行为）。
