# Feature Specification: UI/UX 改版与快速上手（v0.1.1）

**Feature Branch**: `dev/tdy`（specs 工作区：`specs/004-ui-ux-redesign/`）

**Created**: 2026-10-26

**Status**: Draft

**Input**: User description: "v0.1.1：`ccs gui` 与 Wails 客户端 UI/UX 改版（设计系统 + 设置页模块化 + 多视角重设计）；添加服务商流程重设计（预设快速上手、补齐常见服务商、默认最新模型）；远端预设热更新；连通性测试；R11 限额查询显示（5h/周限制，手动刷新）。"

**决策输入**: docs/requirements.md v0.1.1 小节（Q14–Q21 已逐项确认，R11 已定机制）。本 spec 不再保留 [NEEDS CLARIFICATION]——所有开放问题已在可行性分析阶段由用户拍板。

## User Scenarios & Testing *(mandatory)*

> 单一前端双形态（FR-015 沿革）：Web（`ccs gui`）与桌面（Wails）共用同一份前端源码，本 spec 所有故事在两种形态下同时成立（桌面专属内容按既有 404 探测契约条件渲染）。

### User Story 1 - 模块化导航：设置按领域拆分（Priority: P1）

用户打开 GUI，看到的不再是 8 个区块堆叠的一页长表单，而是左侧导航栏（窄屏折叠为顶部 tab）划分的模块：**服务商**（首页，高频）、**模式与路由**、**安全**（Keychain）、**桌面端**（Web 形态隐藏）、**外观与通用**（主题/语言）、**关于与更新**。每个模块只承载该领域的设置；模块内组件可独立维护与扩展。

**Why this priority**: "所有设置在一个页面"是用户明确提出的第一痛点；模块化是本次改版的骨架，后续所有故事都挂在这个骨架上。

**Independent Test**: 在 Web 与桌面两种形态下逐模块走查：每个既有功能（模式切换、路由表、Keychain、桌面设置、主题、语言、更新）都能在对应模块内完成，行为与改版前等价（e2e 用例全绿）。

**Acceptance Scenarios**:

1. **Given** 已有 provider 配置的机器， **When** 打开 GUI， **Then** 落在「服务商」首页，侧边栏可见全部模块入口；桌面形态多出「桌面端」模块，Web 形态没有。
2. **Given** 窄于断点（<880px）的窗口， **When** 渲染界面， **Then** 侧边栏折叠为顶部 tab，所有模块仍可达。
3. **Given** 任一模块， **When** 完成该模块内的设置变更， **Then** 保存行为、校验与错误提示与改版前语义一致（不回归）。

---

### User Story 2 - 设计系统：统一美观的组件与视觉（Priority: P1）

用户感知到界面"变好看了、变一致了"：统一的设计 token（色板/字号/间距/圆角）、统一的基础组件（按钮/输入/卡片/开关/对话框）、lucide 图标替代手写 SVG、toast 通知替代页面顶部永久错误条、空状态引导。深浅色主题延续现有 light/dark/system 机制。

**Why this priority**: "界面丑、不一致"是用户提出的原始诉求；设计系统是美观与一致性的载体，也让后续维护者有章可循。

**Independent Test**: 同一页面在 light/dark/system 三主题下无破损；所有交互反馈走统一组件；既有 40 个 data-testid 语义不变（e2e 不因换皮而失败）。

**Acceptance Scenarios**:

1. **Given** 任一模块页， **When** 切换 light/dark/system， **Then** 全部组件正确响应，无残留硬编码颜色。
2. **Given** 操作产生错误/成功反馈， **When** 反馈出现， **Then** 以 toast 或就地提示呈现，不阻塞其他区域，可自动消失或手动关闭。
3. **Given** 无任何 provider 的新装机， **When** 打开 GUI， **Then** 服务商首页显示空状态引导（指向「从预设添加」）。
4. **Given** Web 与桌面两种形态， **When** 并排对比， **Then** 视觉与交互完全一致（单一前端，无分叉）。

---

### User Story 3 - 添加服务商画廊与极简配置（Priority: P1）

用户点击「添加服务商」，进入**预设画廊**（按分类分组的品牌卡片：官方/国内官方/聚合器/云厂商/自定义）。选择一张卡片后进入分步表单：**第一步只需填 API key**（名称已给默认值，取 key 的链接就地展示）；高级配置（模型覆盖、额外环境变量、隔离模式）折叠在第二步。全程不接触 JSON。保存即得到开箱即用的 provider（预设已带最新默认模型）。

**Why this priority**: 降低上手门槛是追加需求的核心——"选服务商 + api-key 就可以用"；这是本次 UX 重设计的旗舰任务流。

**Independent Test**: 新用户路径：画廊选 DeepSeek → 粘 key → 保存 → `ccs use` 可用；全程 ≤3 步、0 次 JSON 编辑（e2e 可测）。

**Acceptance Scenarios**:

1. **Given** 预设画廊， **When** 按分类浏览并选择某服务商卡片， **Then** 进入分步表单，仅需 API key 即可保存。
2. **Given** 分步表单， **When** 展开「高级配置」， **Then** 可覆盖默认模型（预设默认值可见）与其他 env。
3. **Given** 编辑既有 provider， **When** 打开表单， **Then** 沿用同一表单组件，回显现有配置（编辑路径不回归）。
4. **Given** 预设声明的必填项缺失（如 API key 为空）， **When** 保存， **Then** 就地校验拦截并指明字段，不发请求。

---

### User Story 4 - 预设扩充与默认模型保鲜（Priority: P2）

预设覆盖常见服务商：在既有 13 个基础上补齐 **MiniMax**、**通义千问（百炼）**（均已核实 Anthropic 兼容端点），并复核全部既有预设的默认模型为各家当前旗舰（sonnet 位）/性价比档（haiku 位）。核实不过关的候选（豆包/Groq/Cerebras/Ollama）不硬上。

**Why this priority**: 画廊的体验上限由预设质量决定——预设不全或默认模型过时，极简配置就名不副实。

**Independent Test**: 每个新预设的端点/模型名有调研记录（spec 契约文档）；`presets_test.go` 锁定模板完整性（必填字段、模型键齐全）。

**Acceptance Scenarios**:

1. **Given** 画廊， **When** 浏览「国内官方」分类， **Then** 可见 MiniMax 与通义千问卡片，选中后模板含正确 Base URL 与默认模型。
2. **Given** 任一预设， **When** 应用， **Then** 生成的 env 含 `ANTHROPIC_MODEL` 与三档 `ANTHROPIC_DEFAULT_*_MODEL`（官方/特殊预设除外）。

---

### User Story 5 - 远端预设热更新（Priority: P2）

预设清单不再只随版本发布：GUI 启动时异步拉取本仓库托管的远端 `presets.json`（24h TTL + 手动「刷新预设」），校验通过后生效；离线/失败/校验不过时回落本地内置，界面明示预设来源与更新时间。安全约束：地址锁定本仓库、仅 HTTPS、schema 校验、偏好可关闭远端。

**Why this priority**: Q19 用户明确要求本期即做 B——模型迭代快，预设保鲜不能等发版。

**Independent Test**: 用本地 HTTP 服务模拟远端：正常更新/404/超时/字段损坏四种情形各有确定性结果（单测覆盖）；关闭开关后零网络请求。

**Acceptance Scenarios**:

1. **Given** 联网且远端有更新， **When** GUI 启动或点击「刷新预设」， **Then** 画廊呈现新清单，显示「远端 · 更新于 <时间>」。
2. **Given** 离线/远端不可达/JSON 校验失败， **When** 启动， **Then** 使用内置或上次有效缓存，界面提示来源为本地。
3. **Given** 偏好关闭远端预设， **When** 启动或刷新， **Then** 不发起任何网络请求。
4. **Given** CLI `ccs add -i`， **When** 交互式列预设， **Then** 与 GUI 看到同一份（合并后的）清单。

---

### User Story 6 - 连通性测试（Priority: P2）

添加/编辑表单与 provider 详情提供「测试连接」：后端用当前表单的 Base URL + key + 默认模型发一个 max_tokens=1 的真实请求，返回 成功（含延迟与模型回显） 或结构化失败原因（key 无效/地址不通/模型不存在）。仅手动触发。

**Why this priority**: Q20 确认纳入——把"保存后发现配错"提前为"保存前就知道配错"；与 R11 共用同一 HTTP 探测基建。

**Independent Test**: 后端单测以 httptest 模拟 providers 各类响应（200/401/404/超时），断言结构化结果；前端按钮态（空闲/进行中/成功/失败）有 vitest 覆盖。

**Acceptance Scenarios**:

1. **Given** 表单填写完整， **When** 点击「测试连接」， **Then** 显示进行中态，结果返回后显示成功（延迟/模型）或分类错误。
2. **Given** 错误的 API key， **When** 测试， **Then** 明确提示认证失败而非笼统网络错误。
3. **Given** 表单必填项缺失， **When** 测试， **Then** 按钮禁用或就地校验拦截。

---

### User Story 7 - 限额查询显示（R11，Priority: P3）

用户在 GUI 内查看 provider 的用量限额：支持的服务商显示 5 小时窗口与周窗口的 已用/上限/重置时间（进度条 + 倒计时）；不支持的显示「该服务商不支持限额查询」。查询由后端发起（token 不出后端），**仅手动刷新**，结果带时间戳缓存。

**Why this priority**: R11 是新增能力，依赖 M2 的探测基建；首批只承诺实测适配的服务商，通用头探测覆盖其余。

**Independent Test**: 后端单测以 httptest 模拟带 `anthropic-ratelimit-unified-*` 头的响应与 GLM/Kimi 专用端点响应，断言归一化窗口数据；不支持的 provider 断言返回 unsupported。

**Acceptance Scenarios**:

1. **Given** 支持限额查询的 provider（如 GLM）， **When** 点击「刷新用量」， **Then** 显示 5h/周窗口的已用、上限与重置时间。
2. **Given** 不支持的 provider， **When** 查看用量区， **Then** 显示「不支持查询」占位，无报错。
3. **Given** 查询失败（网络/服务端错误）， **When** 刷新， **Then** 显示错误与重试入口，不影响其他区域。

---

### Edge Cases

- 深浅色切换中途打开对话框/toast：主题变量全部走 CSS 变量，无闪烁残留。
- 窄屏（<880px）下画廊与分步表单：卡片网格降为单列，步骤导航保持可用。
- 远端预设与内置 id 冲突/远端条目字段缺失：校验拒绝整份远端文件，回落本地（不做逐条合并）。
- 同名 provider 重复创建：沿用现有唯一性校验（id 冲突拦截）。
- 连通性测试/限额查询请求超时：显式超时（探测 10s），失败归类为网络错误，不悬挂 UI。
- `ccs gui` 无网络环境：画廊用内置预设完全可用，远端状态显示离线，无报错弹窗。
- 既有 `providers.json` 用户升级到新版：零迁移（数据模型不变），改版只动 UI/预设层。
- 桌面端窗口最小尺寸（880×600）：模块导航折叠为顶部 tab 后仍完整可用。
- i18n：新增文案 en/zh 双语全覆盖；zh 文案长度变化不破版（弹性布局）。

## Requirements *(mandatory)*

### Functional Requirements

**导航与模块化**

- **FR-001**: GUI MUST 以侧边栏导航划分模块（服务商/模式与路由/安全/桌面端/外观与通用/关于与更新），窄于 880px MUST 折叠为顶部 tab。
- **FR-002**: 模块划分 MUST 保持既有全部功能的可达性与语义（零功能回归）；「桌面端」模块沿用以 `/desktop/info` 404 探测的条件渲染契约（specs/003 FR-015）。
- **FR-003**: 界面 MUST 由单一前端源码构建（Web 与桌面共用），MUST NOT 分叉第二套前端。

**设计系统**

- **FR-004**: MUST 建立统一设计 token（色板/字号/间距/圆角/阴影/动效时长），并支持 light/dark/system 三态主题（延续既有 `data-theme` 机制与偏好存储）。
- **FR-005**: 基础组件 MUST 使用统一组件库体系（Radix UI 无头原语 + Tailwind + lucide-react 图标），组件源码收于前端仓库内（shadcn/ui 模式）。
- **FR-006**: 操作反馈 MUST 使用 toast/就地提示组件；错误 MUST 分类呈现（校验/网络/服务端），MUST NOT 以整页横幅堆积。
- **FR-007**: 服务商列表为空时 MUST 显示空状态引导（指向预设画廊）。
- **FR-008**: 全部既有 `data-testid` MUST 保留且语义不变（e2e 兼容），新增交互添加新 testid。

**添加流程与预设**

- **FR-009**: 添加服务商 MUST 经预设画廊（分类分组 + 品牌卡片），选择后进入分步表单；第一步 MUST 仅要求预设声明的必填项（通常为 API key），高级配置（模型/env 覆盖）MUST 默认折叠。
- **FR-010**: 预设清单 MUST 补齐 MiniMax 与通义千问（Anthropic 兼容端点已核实），并复核既有预设默认模型至各家当前旗舰（sonnet 位）与 flash 档（haiku 位）。
- **FR-011**: 每个预设模板 MUST 含 `ANTHROPIC_BASE_URL` 与默认模型键（官方 OAuth/特殊预设除外），必填项 MUST 有声明。

**远端预设**

- **FR-012**: 系统 MUST 支持从本仓库托管的远端 `presets.json` 更新预设（启动异步拉取 + 24h TTL + 手动刷新），并 MUST 经 schema 校验后才启用。
- **FR-013**: 远端拉取失败/超时/校验失败时 MUST 回落内置预设或上次有效缓存，界面 MUST 显示预设来源与时间。
- **FR-014**: 远端预设地址 MUST 硬编码锁定本仓库（仅 HTTPS）；用户偏好 MUST 提供关闭开关，关闭后 MUST 零网络请求。
- **FR-015**: CLI（`ccs add` 交互式列表）与 GUI MUST 消费同一合并后的预设清单（共享层实现）。

**连通性测试**

- **FR-016**: 后端 MUST 提供 provider 探测端点：以给定 Base URL/key/模型发起 max_tokens=1 的最小请求，返回结构化结果（ok/延迟/模型回显 或 分类错误：auth/network/model/unknown）。
- **FR-017**: 探测 MUST 仅由用户手动触发（前端按钮），MUST NOT 在页面加载或保存流程中自动发起；超时 MUST ≤10s。

**限额查询（R11）**

- **FR-018**: 后端 MUST 提供 provider 用量查询端点：优先透传解析 Anthropic 系限流响应头（`anthropic-ratelimit-unified-*` 等），服务商专用配额端点按适配器接入（首批 GLM Coding Plan、Kimi）；返回归一化窗口列表（类型/已用/上限/重置时间）或明确的不支持标记。
- **FR-019**: 用量查询 MUST 仅手动刷新，MUST NOT 自动轮询；查询 MUST 由后端发起，token MUST NOT 经前端。
- **FR-020**: 不支持的服务商 MUST 返回 unsupported 并在 UI 优雅降级（占位提示，非报错）。

**横切**

- **FR-021**: 全部新增用户可见文案 MUST 提供 en/zh 双语。
- **FR-022**: 改版后 Web（三主流浏览器）与桌面（macOS WKWebView / Windows WebView2 / Linux webkit2gtk）MUST 正常渲染；所用 CSS 特性 MUST 兼容目标 webview 基线（Tailwind 版本据此选定）。
- **FR-023**: 三平台（macOS/Linux/Windows）MUST 行为一致；文件/路径/网络代码 MUST 显式考虑平台差异。

### Key Entities *(include if feature involves data)*

- **Preset（预设模板）**: 既有实体扩展——增加远端来源元数据（source: builtin/remote、fetchedAt）；新增服务商条目（MiniMax/通义千问）。
- **PresetBundle（远端预设清单）**: `presets.json`（schemaVersion + presets[]），托管本仓库，拉取后整体校验、整体生效或整体回落。
- **ProbeResult（连通性测试结果）**: ok、latencyMs、model（回显）、error{kind: auth|network|model|unknown, message}。
- **UsageReport（用量报告）**: supported、provider id、windows[]{type: 5h|week|…, used, limit, resetAt}、fetchedAt、error?。
- **UsageAdapter（限额适配器）**: per-provider 查询策略接口（通用头探测 / 专用端点），新增服务商以适配器扩展。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 新用户从打开 GUI 到完成第一个可用 provider 配置 ≤3 步（选卡 → 粘 key → 保存），全程不接触 JSON（e2e 断言）。
- **SC-002**: 既有 12 条 e2e 用例全部通过（零功能回归），且全部既有 data-testid 保留。
- **SC-003**: 任一模块页在 light/dark/system 三主题与 <880px 窄屏下无布局破损（人工走查 + 响应式断点单测）。
- **SC-004**: 远端预设四情形（更新/不可达/损坏/关闭）行为符合 FR-012–014（后端单测全覆盖）。
- **SC-005**: 支持的服务商一次手动点击即可见 5h/周限额及重置时间；不支持的明确显示不支持（集成测试断言）。
- **SC-006**: 添加流程中的配置错误（key/地址/模型）在保存前经「测试连接」可发现（探测端点单测覆盖 200/401/404/超时）。
- **SC-007**: 桌面端三平台打包产物界面正常（发布流水线既有 matrix + 人工验收清单）。

## Assumptions

- 数据模型（providers.json / profiles）不变，零迁移；改版仅触及前端与预设/探测/用量等新后端能力。
- MiniMax、通义千问的 Anthropic 兼容端点与模型名按 2026-10-26 调研为准，spec 契约文档记录核实来源；服务商若变更端点，由远端预设机制兜底修复。
- GLM/Kimi 限额专用端点以公开资料与官方插件行为为准调研；若实现期发现不可靠，R11 首批范围可收缩为「Claude 官方 + 通用头探测」而不阻塞发版（已在风险中记录）。
- 「测试连接」与「刷新用量」每次产生一次真实最小请求（计费影响可忽略）；不做自动后台探测。
- 快捷键、命令面板、`ccs usage` CLI 命令等为 backlog，不在本期范围。
- Tailwind 版本（v4 或降级 3.4）在基建任务中经兼容性评估后锁定，锁定结论回填本 spec 的契约文档。
