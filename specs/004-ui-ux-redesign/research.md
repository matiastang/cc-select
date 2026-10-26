# Research: UI/UX 改版与快速上手（v0.1.1）

**Date**: 2026-10-26 | **Spec**: [spec.md](./spec.md)

## 1. 现状盘点（代码级）

| 事实 | 出处 | 含义 |
|---|---|---|
| 单一前端双形态：桌面 = Wails 壳复用 `web.Handler()`，叠加 update/desktop 桥 | `desktop/main.go:62` | 改一份前端，两形态同时生效；FR-003 天然满足 |
| App.tsx 8 区块垂直堆叠，无路由 | `internal/frontend/src/App.tsx` | 模块化 = 新增导航层 + 特性目录 |
| 自研 ui 基件 10 个（多为 7–12 行薄包装）、手写 SVG 图标 12 个、559 行 style.css | `src/components/ui/`、`src/style.css` | 组件库替换面小、迁移成本可控 |
| 主题机制：`data-theme` + `prefers-color-scheme`，localStorage 存偏好 | `src/theme/theme.ts` | 保留机制，token 映射到 CSS 变量 |
| src 内 40 个 `data-testid`，e2e 4 spec 12 条用例 | `src/`、`e2e/` | FR-008 硬约束：testid 全保留 |
| 预设 13 个，Go 字面量编译进二进制，模板已含默认模型 | `internal/presets/presets.go` | 单源化改造（见 §3.2） |
| CLI `ccs add` 与 Web `/api/v1/presets` 都消费 `internal/presets` | `internal/cli/add.go:123`、`internal/web/api.go:90` | 远端预设逻辑必须放共享层 |
| Web 资产 272KB embed | `internal/web/assets/` | 组件库体积增长可接受 |
| `ccs gui` 默认端口 7799；vite dev 代理 `/api` → 7799 | `vite.config.ts` | dev 流不变 |
| prefs.json 已有结构化偏好 + `DesktopPrefs` 先例（新增字段向后兼容模式） | `internal/prefs/prefs.go` | 新增 `remotePresetsDisabled` 沿用同模式 |

## 2. 外部事实核实（2026-10-26，Phase 0 调研）

### 2.1 新预设服务商（Q18）

| 服务商 | Base URL | 认证 | 模型（当前） | 来源 |
|---|---|---|---|---|
| **MiniMax** | `https://api.minimax.io/anthropic` | `ANTHROPIC_AUTH_TOKEN`（Bearer 兼容 x-api-key） | `MiniMax-M3`（旗舰）、`MiniMax-M3.1-Flash` | [官方 Anthropic 兼容文档](https://platform.minimax.io/docs/api-reference/text-anthropic-api)；仅支持这两个模型；CN 站未在文档出现，首发用国际站，CN 域名列入远端预设兜底演进 |
| **通义千问（百炼 Coding Plan）** | `https://coding.dashscope.aliyuncs.com/apps/anthropic` | `ANTHROPIC_AUTH_TOKEN` | 全档 `qwen3.7-plus`（官方推荐全映射） | [百炼 Claude Code 官方文档](https://help.aliyun.com/zh/model-studio/claude-code)；另支持 `qwen3.7-plus[1m]` 1M 上下文后缀 |
| 通义千问（按量计费，备选） | `https://dashscope.aliyuncs.com/apps/anthropic` | 同上 | 主/Sonnet/Opus `qwen3.7-max`、Haiku `qwen3.6-flash` | 同上；首发收录 Coding Plan 版（与用户"快速上手"诉求最贴），按量版列 backlog |

### 2.2 既有预设模型保鲜（Q21）

| 预设 | 现值 | 动作 | 依据 |
|---|---|---|---|
| 智谱 GLM（中/英） | `glm-5.1` 全档 | → 主/Sonnet/Opus `glm-5.3`、Haifu→Haiku `glm-5.3-flash` | GLM-5.3 为当前旗舰（2026-08-19 API 上线），变体 Flash/FlashX；[官方模型页](https://docs.bigmodel.cn/cn/guide/models/text/glm-5.3)；沿用既有小写命名惯例 |
| DeepSeek | `deepseek-v4-pro/flash` | 保持（当前即 V4 系列） | [官方接入文档](https://api-docs.deepseek.com/zh-cn/quick_start/agent_integrations/claude_code) |
| Kimi | `kimi-k2.7-code` | 保持（v0.1.0 核实） | Kimi 开放平台当前通用 K2.6 / Coding 线 k2.7-code |
| OpenRouter | claude-sonnet-5 系 | 保持 | — |

### 2.3 限额查询机制（R11）

**通用头探测（Anthropic 系）**：`/v1/messages` 响应头 `anthropic-ratelimit-unified-*` 携带并发/5h/7d 窗口的 used/limit/reset（分号分隔多窗口条目）；另有分钟级 `anthropic-ratelimit-requests-*/tokens-*`。Claude Code 自身不持久化显示（官方 issue #55333）。实现按「防御式解析」：同时识别独立头与多值头，零窗口可得时判 unsupported。来源：[限流头解析说明](https://www.ssdnodes.com/learn/claude-api-rate-limit-headers-explained)、[官方 Rate Limits](https://platform.claude.com/docs/en/api/rate-limits)。

**GLM Coding Plan（专用端点，已从官方插件源码核实）**：
- `GET https://{open.bigmodel.cn|api.z.ai}/api/monitor/usage/quota/limit`（无查询参数）
- 认证：`Authorization: <ANTHROPIC_AUTH_TOKEN>`（原样，无 Bearer 前缀）
- 响应：`data.limits[]`，字段 `type/percentage/currentValue/usage/usageDetails`；`TOKENS_LIMIT` = 5 小时窗口（百分比）、`TIME_LIMIT` = 月度 MCP 用量。**无周限额与重置时间字段**——归一化模型必须允许窗口缺省字段（percent-only）。
- 来源：[zai-org/zai-coding-plugins](https://github.com/zai-org/zai-coding-plugins) `plugins/glm-plan-usage/skills/usage-query-skill/scripts/query-usage.mjs`（Apache-2.0）。

**Kimi**：开放平台有余额端点（`GET /v1/users/me/balance`，Bearer）；`api.kimi.com/coding`（For Coding）无公开用量端点。**决策**：Kimi 首批仅对开放平台 host 提供余额查询；coding host 标记 unsupported。诚实兑现 spec 的收缩条款。

**MiniMax/通义**：未核实到公开限额端点 → 走通用头探测，大概率 unsupported → UI 优雅降级（符合 FR-020）。

### 2.4 Tailwind 版本决策（Q14 遗留项，此处锁定）

**定 Tailwind 3.4.x，不用 v4**。理由：v4 依赖 `@property`/cascade layers/`color-mix()`，基线约 Safari 16.4+；目标 webview 里 macOS WKWebView（macOS 13 低小版本 = Safari 16.1–16.3）与部分 Linux webkit2gtk 发行版落在基线之下，破坏 FR-022。v3.4 支持 Safari ≥14，且 shadcn/ui 组件模式对 v3/v4 无差别（组件源码收仓库自有，不依赖 v4 特性）。Wails v3 将来抬高最低基线时可再评估升级。

## 3. 关键技术决策

### 3.1 前端栈（M1）

- Tailwind 3.4 + PostCSS/autoprefixer（v3 标配）+ `clsx` + `tailwind-merge`（`cn()` 合并工具）+ `tailwindcss-animate`（Radix 动画类）。
- Radix 无头原语按需：dialog、tabs、switch、select、tooltip、label、dropdown-menu、separator、toast（`react-aria-components` 等替代方案否决——与 shadcn/ui 模式绑定 Radix，Q14 已定）。
- `lucide-react` 图标，替换 `ui/Icon.tsx` 手写 SVG（Icon 组件保留为转发兼容层，逐步退役）。
- 路由：自研 hash router（~60 行，`src/navigation/`），**不引 react-router**（6 个固定模块 + 条件注册桌面模块，YAGNI；hash 保证 Web 端后退/深链）。
- 设计 token：CSS 变量（延续现有 `--bg/--card/...` 命名族）+ Tailwind `theme()` 映射——token 单点定义，组件只消费语义类。

### 3.2 预设单源化（M2 前置，M1 落地数据）

- 预设从 Go 字面量迁到 **`internal/presets/presets.json`**，`//go:embed` 进二进制作为兜底源；远端地址 = 同一文件在 GitHub main 分支的 raw URL（`https://raw.githubusercontent.com/matiastang/cc-select/main/internal/presets/presets.json`）。**一份文件两个分发面**，杜绝 Go/JSON 双源漂移。
- `internal/presets` 重构为：类型 + 校验器 + 源解析器（builtin → cache → remote，含 TTL/手动刷新/禁用开关），`All()/ByID()/BuildEnv()` 对外 API 不变（CLI/Web 零改动受益）。

### 3.3 后端新能力（M2/M3）

- `internal/probe`（新）：连通性测试——最小 `/v1/messages` 请求（max_tokens=1），结构化结果（ok/latency/model 或 auth/network/model/server/unknown 分类）。超时 10s，仅手动触发。
- `internal/usage`（新）：限额查询——`Adapter` 接口 + 注册表（按 base URL host 匹配；默认落通用头探测适配器）；GLM 专用适配器 + Kimi 余额适配器。归一化 `Window` 允许 percent-only（GLM 事实）。
- 两者共享一个最小 Anthropic 客户端封装（发请求、读头、分类错误）。

### 3.4 REST 契约增量（详见 contracts/api.md）

- `GET /api/v1/presets` 响应增补 `meta{source,fetchedAt}`（向后兼容）
- `POST /api/v1/presets/refresh`（手动刷新）
- `GET|PUT /api/v1/remote-presets`（开关）
- `POST /api/v1/providers/probe`（连通性测试）
- `POST /api/v1/usage/{id}`（限额查询，仅手动）

## 4. 被否决的替代方案

| 方案 | 否决理由 |
|---|---|
| Tailwind v4 | webview 基线风险（§2.4） |
| react-router | 6 模块固定导航用不上其能力；自研 hash router 60 行足够（YAGNI） |
| react-query/SWR | 请求面小且已有 fetch 模式；引入服务状态库不成比例 |
| 整库组件库（Mantine/AntD） | Q14 已定 shadcn/ui 模式；整库替换体积大、主题锁定 |
| 预设双源（Go 字面量 + 远端 JSON） | 必然漂移；单源文件 + embed 一致性最好 |
| ccusage 式本地日志估限额 | 非真实限额数据；R11 诉求是服务商侧真值 |
| 远端预设逐条合并 | 复杂且部分损坏难判定；整包校验、整包生效/回落，语义简单可测 |
