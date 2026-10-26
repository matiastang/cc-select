# Implementation Plan: UI/UX 改版与快速上手（v0.1.1）

**Branch**: `dev/tdy` | **Date**: 2026-10-26 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/004-ui-ux-redesign/spec.md`；决策输入 docs/requirements.md v0.1.1（Q14–Q21 + R11）。

## Summary

以**模块化导航 + 设计系统**重构单一前端（Web/桌面双形态共用），把「添加服务商」重设计为**画廊 + 分步向导**（选卡 → 粘 key → 保存）；预设**单源化**（JSON embed + GitHub raw 远端热更新，整包校验/回落）；新增**连通性测试**与 **R11 限额查询**（通用限流头探测 + GLM/Kimi 专用适配器，仅手动触发）。后端新增 `internal/probe`、`internal/usage` 两包与 5 个 REST 端点；`providers.json`/profiles 数据模型零变更。

## Technical Context

**Language/Version**: Go 1.26（CLI + 桌面壳）；前端 TypeScript 5.7 + React 18 + Vite 5

**Primary Dependencies（新增）**: tailwindcss@^3.4（+postcss/autoprefixer/tailwindcss-animate）、Radix UI 无头原语（9 个）、lucide-react、clsx + tailwind-merge；Go 侧零新依赖（标准库 net/http）

**Storage**: 既有 JSON 两层存储不变；新增 `~/.cc-select/presets.cache.json` + `prefs.remotePresetsDisabled` 字段 + `internal/presets/presets.json`（embed 单源）

**Testing**: Go 单测（table-driven + httptest）+ 既有集成测试 + vitest 组件测试 + Playwright e2e（既有 12 条保绿 + 新增 5 spec）

**Target Platform**: macOS/Linux/Windows 三平台；Web（主流浏览器）+ 桌面三 webview（WKWebView/WebView2/webkit2gtk，CSS 基线 Safari 15）

**Project Type**: Go 单二进制 CLI + 嵌入式 Web GUI + Wails v3 桌面壳

**Performance Goals**: 首屏 JS gzip 增量 ≤ 150KB；probe/usage 单次 ≤ 10s 超时；远端预设拉取 5s 超时不阻塞首屏（异步）

**Constraints**: 单一前端不分叉（specs/003 FR-015 沿革）；40 个既有 data-testid 语义不变；`make check` 全绿；一功能一 commit；TDD

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| 原则 | 判定 | 说明 |
|---|---|---|
| I. Eval/Wrapper 拆分 | ✅ PASS | 不触及 shell 发射/`ccs` wrapper |
| II. CLAUDE_CONFIG_DIR 隔离 | ✅ PASS | use/profile/路由机制不动；预设默认模型仅供 modelPicker 注入（R10 既有链路） |
| III. 三平台 | ✅ PASS | 远端拉取/缓存走标准库 + 既有 config 目录；CSS 基线 Safari 15（Tailwind 3.4 定版理由 research §2.4）；三平台桌面人工验收入 quickstart |
| IV. 选型不漂移 | ⚠️ 有据扩展 | 新增前端依赖 15 项系 Q14 用户已确认决策（docs/requirements.md 已先于代码更新，满足修订流程）；Go 侧零新依赖。详见 Complexity Tracking |
| V. 质量门禁与验收同步 | ✅ PASS | `make check` 扩展 presets-lint；acceptance-tests.md 增 AC19（T018） |
| VI. 安全基线 | ✅ PASS | 无新凭证存储路径；远端预设锁定仓库+HTTPS+拒绝跨 host 重定向；probe/usage 错误透传剥 token |
| VII. 文档链真值 | ✅ PASS | T018 回流 docs/（requirements 变更记录、tech-stack、roadmap、acceptance-tests、CLAUDE.md）；宪法 Sync Report 补依赖事实 |
| VIII. TDD | ✅ PASS | 每任务先红后绿；tasks.md 显式标注测试先行项 |

## Project Structure

### Documentation (this feature)

```text
specs/004-ui-ux-redesign/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── api.md
│   ├── frontend.md
│   └── presets-and-usage.md
├── checklists/requirements.md
└── tasks.md            # /speckit-tasks 产出
```

### Source Code (repository root)

```text
internal/
├── presets/
│   ├── presets.go          # 重构：类型 + 对外 API（All/ByID/BuildEnv 不变）
│   ├── presets.json        # 新：单源预设（embed + GitHub raw 双分发）
│   ├── validate.go         # 新：整包校验器
│   ├── source.go           # 新：builtin→cache→remote 解析 + Refresh + Meta
│   └── *_test.go
├── probe/                  # 新：连通性测试（共享最小 Anthropic 客户端）
│   ├── client.go           #   SendMessage + 错误分类（probe/usage 共用）
│   ├── probe.go
│   └── *_test.go
├── usage/                  # 新：限额查询
│   ├── usage.go            #   Report/Window/Adapter 接口 + 注册表
│   ├── anthropic_headers.go
│   ├── glm_quota.go
│   ├── kimi_balance.go
│   └── *_test.go
├── web/api.go              # 增 5 端点 + presets meta
├── prefs/prefs.go          # 增 remotePresetsDisabled
└── frontend/src/           # 结构演进见 data-model §3.2
```

**Structure Decision**: 沿用仓库既有 Go 布局；前端在既有 `src/` 内演进（新增 `navigation/`、`api/`、`features/`），不搬迁目录以保 e2e/构建链稳定。

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| 前端新增 15 依赖（tailwind/radix/lucide/cn 工具族） | Q14 用户决策「引入三方库保证组件美观与一致性」；shadcn/ui 模式是组件源码自有 + 按需体积的最小实现 | 继续手写 CSS/自研基件——无法满足一致性诉求，且维护成本更高（research §4） |
| 预设 Go 字面量 → JSON 单源 + embed | 远端热更新（Q19）要求运行时可整体替换清单；单文件双分发（embed + raw URL）是唯一无漂移结构 | Go+JSON 双源同步——必然漂移；运行时仍编译死——不满足 Q19 |
| 自研 hash router（~60 行） | 6 固定模块 + 条件注册；hash 已满足 Web 后退/深链 | react-router——能力过剩（YAGNI，宪法技术约束「依赖从简」） |

## Milestones

| 里程碑 | 内容 | 交付判据 |
|---|---|---|
| **M1 设计系统与 UX 改版** | Tailwind/token/基件、布局壳+导航、六模块迁移、toast/空状态、画廊+向导、预设单源化数据迁移（MiniMax/通义/GLM 保鲜） | 既有 e2e 全绿；向导 ≤3 步建 provider（新 e2e）；深浅色/窄屏走查通过 |
| **M2 远端预设与连通性测试** | source 解析状态机、refresh/开关端点与 UI、CI presets-lint、probe 端点与「测试连接」 | 四情形（更新/不可达/损坏/禁用）单测全覆盖；probe 分类单测全覆盖 |
| **M3 限额查询（R11）** | usage 框架 + 3 适配器 + 端点 + 用量面板 | GLM/Kimi/通用头 httptest 断言；unsupported 优雅降级 e2e |
| **收尾** | 全量回归、循环 code review、文档回流、三平台桌面验收 | `make check` + 全测试绿；无中等及以上评审遗留；docs 链更新 |

## Risks & Mitigations（承 spec Assumptions + feasibility 风险清单）

1. **Radix/Tailwind 与老 webkit2gtk 的边角兼容**：组件基线选型保守（Tailwind 3.4）；quickstart §4 三平台人工清单为发布门禁。
2. **GLM/Kimi 端点无 SLA 承诺**：适配器隔离 + unsupported 降级；M3 允许收缩首批范围（spec Assumptions 第 3 条）。
3. **e2e 因导航结构失效**：默认路由 `#/providers` + testid 保留契约（contracts/frontend §5），预期仅补导航步骤。
4. **远端预设仓库文件被误改**：CI presets-lint 用运行时同一校验器拦截。
5. **范围膨胀**：里程碑串行、一功能一 commit；backlog（快捷键/命令面板/ccs usage）不进本期。
