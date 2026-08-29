# Feature Specification: Mode P 会话内真实模型选择与显示（model-picker-sync）

**Feature Branch**: `002-model-picker-sync`

**Created**: 2026-08-29

**Status**: Draft

**Input**: User description: "v0.0.7 / R10：Mode P 下在 Claude Code 内查看并切换当前 provider 真实模型（modelPicker 注入 / 热切跟随刷新 / 代理映射化改写）。需求已录入 docs/requirements.md「### v0.0.7」，可行性已实验定案（2026-08-29 三轮探针实验：CC ≥ 2.1.242 的 modelPicker 支持会话内热重载，provider 格式 id 不被行审查丢弃）"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 启动会话即见真实模型（Priority: P1）

用户在某个终端执行切换命令进入 Claude Code 后，打开 `/model`，看到的是**当前服务商实际配置的模型**（如 GLM 5.3 / GLM 5.3 Flash），而不是与当前服务商无关的内置 Anthropic 模型目录（含误导性定价标签）。当前正在使用的模型行有正确标记。

**Why this priority**: 这是本特性的可见核心——「看到真实模型」是用户提出本需求的直接动机（现状显示错误模型名，含错误定价，造成持续误导）。且该能力在 Mode B 下同样生效，独立交付即有价值。

**Independent Test**: 全新终端 `use <provider>` 后进入 Claude Code，打开 `/model`，列表仅含该 provider 配置的模型，当前模型 ✔ 正确；官方 provider（无 env）场景不注入、行为与现状一致。

**Acceptance Scenarios**:

1. **Given** provider 的 env 配置了 `ANTHROPIC_MODEL` 与 `ANTHROPIC_DEFAULT_*_MODEL`，**When** 用户 `use` 该 provider 并进入 Claude Code 打开 `/model`，**Then** 列表显示去重后的全部已配置模型（provider 格式 id 原样显示），不显示内置 Anthropic 目录行。
2. **Given** 同一 provider，**When** 用户在 `/model` 中查看当前模型，**Then** ✔ 落在实际使用的模型行上。
3. **Given** 官方 provider（unset，无 env），**When** 用户进入 Claude Code 打开 `/model`，**Then** 与现状一致（不注入任何内容）。
4. **Given** provider env 未配置任何模型变量，**When** 用户打开 `/model`，**Then** 不注入，保持现状。

---

### User Story 2 - /model 中选择真实模型并生效（Priority: P2）

用户在 `/model` 中选择了服务商配置的另一个模型后，后续对话实际由该模型服务——选择不再被代理无感覆写为服务商主模型。

**Why this priority**: 「能切换」是需求三要素之一；没有它，Story 1 的列表只是装饰。Mode B 下此能力现状已具备（直连），本故事主要补齐 Mode P 代理模式下的缺口（代理统一改写为 main model 的 v1 简化）。

**Independent Test**: Mode P 会话中 `/model` 选择同 provider 的另一个模型，通过服务商侧的调用记录验证后续请求使用该模型 id；未配置模型的 provider 与超大请求体等场景保持 v1 透传行为不变。

**Acceptance Scenarios**:

1. **Given** Mode P 会话路由到 provider A，A 的清单含模型 X 与 Y，当前用 X，**When** 用户在 `/model` 选择 Y，**Then** 下一笔请求的 model 为 Y（服务商侧可验证），不被改写回 X。
2. **Given** 请求携带内置目录模型 id（claude 系，如后台小任务），**When** 代理转发，**Then** 按槽位映射翻译为 A 的对应模型（opus/sonnet/haiku 槽 → 对应 DEFAULT 变量，未配置该变量时回落主模型），与 v1 行为兼容。
3. **Given** 当前 provider 未配置 `ANTHROPIC_MODEL`，**When** 代理转发任意请求，**Then** 原样透传（v1 行为不回归）。

---

### User Story 3 - 热切后选择器跟随新服务商（Priority: P3）

Mode P 会话内热切到另一个服务商后，**不重启会话**，用户打开 `/model` 即看到新服务商配置的模型列表，可选择并生效。

**Why this priority**: 这是「热切后显示跟随」的完整闭环，依赖 Story 1（注入格式）与 Story 2（选择生效）先就位；单独交付价值低于前两者，但它是 Mode P 差异化体验的最后一块。

**Independent Test**: Mode P 双 provider 场景，会话内热切后打开 `/model`，列表为新 provider 模型；在途请求完成后切换，上下文延续（R9 既有验收不回退）。

**Acceptance Scenarios**:

1. **Given** Mode P 会话当前路由到 provider A（清单含模型 X），**When** 会话内热切到 provider B（清单含模型 Z），**Then** 不重启会话，打开 `/model` 显示 B 的模型清单（含 Z），不含 A 的模型。
2. **Given** 热切前用户在 `/model` 选了 A 的模型 X，**When** 热切到 B（B 清单不含 X），**Then** 代理将 X 按回落规则处理（映射不到则回落 B 主模型），请求不失败、不发出 B 不认识的模型 id。
3. **Given** 热切与 Claude Code 自身写 settings 并发发生，**When** 双方各自完成写入，**Then** 双方字段互不丢失（model 字段与选择器配置同时存活）。

---

### Edge Cases

- **provider 无任何模型变量**（只有 BASE_URL/TOKEN）：不注入，选择器保持现状；代理保持透传。
- **模型变量重复**（如 `ANTHROPIC_MODEL` 与 `ANTHROPIC_DEFAULT_SONNET_MODEL` 同值）：注入清单去重，选择器不显示重复行。
- **热切目标清单不含已选模型**：代理回落主模型，不发出目标服务商不认识的 id。
- **写入竞态**：Claude Code 自身会把 `/model` 选择写入 settings（`model` 字段），热切刷新同时写选择器配置——须字段级合并 + 原子写，任何一方不得覆盖另一方。
- **settings.json 损坏或不可写**：热切刷新写失败不得阻断路由切换本身（路由表是 Mode P 真值，选择器显示是增强）；报错并保留现状显示。
- **Claude Code 版本过旧**（无 `modelPicker` 支持版本）：静默跳过注入（CLI 给出一次性升级提示），不影响路由与切换功能。
- **多终端并发**：终端 A 的热切刷新只写终端 A 的配置目录，终端 B 的选择器与路由不受影响（R1 回归红线）。
- **官方 provider 与 Mode A**：官方 provider 不注入；Mode A 的 profile settings.json 同样适用注入规则。

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 系统 MUST 在生成/重建 profile settings.json 时，把当前 provider 的模型清单注入 Claude Code 模型选择器配置：`ANTHROPIC_MODEL` 与 `ANTHROPIC_DEFAULT_{OPUS,SONNET,HAIKU}_MODEL` 去重后为全部选项，且仅显示这些选项（隐藏内置目录）；官方 provider（无 env）与未配置任何模型变量的 provider MUST NOT 注入。
- **FR-002**: 注入的每个模型行 MUST 以可读的模型标识显示（模型 id 原文或等价可读名），且 Claude Code 能识别「当前使用模型」行并正确标记。
- **FR-003**: 代理的模型改写 MUST 从「无条件覆写为主模型」改为映射化规则：请求 model 属于当前路由 provider 清单 → 原样转发；内置目录 model id（含 opus/sonnet/haiku 别名）→ 按槽位映射到 provider 对应模型变量，该变量未配置时回落主模型；provider 未配置 `ANTHROPIC_MODEL` 时 → 保持原样透传（v1 行为）。
- **FR-004**: Mode P 热切 MUST 同步刷新该终端配置目录下的选择器配置为新 provider 的模型清单；刷新 MUST 以读-改-写合并方式执行，保留 Claude Code 写入的其他字段（如 `/model` 选择持久化的 `model` 字段），且写入 MUST 是原子的。
- **FR-005**: 热切刷新 MUST 只影响发起切换的终端（经该终端配置目录定位），其他终端的选择器配置与路由 MUST 不变。
- **FR-006**: 当用户配置了 `availableModels` 白名单时，注入清单 MUST 与之兼容（白名单不得拒绝注入的模型行）。
- **FR-007**: 检测到 Claude Code 版本不支持选择器配置时，系统 MUST 跳过注入并提示升级，不影响其余功能。
- **FR-008**: 注入能力 MUST 与隔离模式无关：Mode A / Mode B / Mode P 的 profile settings.json 适用同一规则。

### Key Entities *(include if feature involves data)*

- **模型清单（provider 派生）**:某 provider 可供选择的模型集合，每个条目含模型 id、来源槽位（主模型 / opus / sonnet / haiku）、展示名；由 provider env 派生，是注入与代理映射的共同数据源。
- **选择器配置块**:profile settings.json 内由本系统维护的配置段（模型行列表 + 「仅显示这些行」开关）；与 Claude Code 自有的 `model` 字段等其他字段共存。
- **槽位映射规则**:内置目录模型 id → provider 模型变量的翻译表（opus 槽 / sonnet 槽 / haiku 槽 / 缺省 → 主模型），代理转发时使用。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 用户进入 Claude Code 后打开模型选择器，100% 看到当前服务商已配置的全部模型（无遗漏、无重复、无无关内置目录行），且能在 1 次操作内确认当前使用模型。
- **SC-002**: 热切到另一服务商后，不重启会话，100% 复现「打开模型选择器即见新服务商模型清单」。
- **SC-003**: 用户在选择器中选择某模型后，下一笔模型请求 100% 使用该模型（以服务商侧调用记录为准验证），0 例被静默改写回主模型。
- **SC-004**: R1 隔离回归红线：任一终端热切/选择模型，其他终端的路由与显示 0 受影响（双终端并发场景 100% 通过）。
- **SC-005**: 并发写入场景（Claude Code 写 `model` 字段与本系统写选择器配置）下，0 字段丢失、0 配置文件损坏。

## Assumptions

- **目标 Claude Code 版本 ≥ 2.1.242**（用户环境已升级 2.1.251；选择器配置 `modelPicker` 需该版本起）。旧版本策略采用 requirements Q12 默认倾向：跳过注入 + 提示升级，不做单行自定义选项兜底。
- **模型清单来源采用 requirements Q11 默认倾向**:v0.0.7 从 env 四个模型变量派生（最多 4 行，零 schema 变更）；provider 显式 `models` 字段（任意多模型 + 自定义名/描述）留作后续增强，不在本期。
- **热切写路径采用 requirements Q13 倾向**:`route switch` 由 CLI 在会话内执行，经继承的 `CLAUDE_CONFIG_DIR` 定位该终端配置目录直接改写；不引入 TID→profile 目录映射登记。三平台 env 继承行为列入 plan 阶段研究验证项。
- **用户在选择器中的选择遵循 Claude Code 原生语义**（确认=存为默认、单次键=仅本会话），本系统不额外干预或另建持久化。
- **不做 gateway model discovery 集成**（2026-08-29 实验判为不可行：仅启动查询一次、结果缓存、且模型 id 须经 claude/anthropic 过滤）。
- **Mode P 下选择器显示与热切路由的分离语义维持设计现状**：显示刷新尽力而为（写失败不阻断路由），路由真值始终是 daemon 路由表。
- 注入不触碰密钥：模型清单与密钥无关，profile settings.json 的 Mode P 无密钥约束（宪法原则 II）不受影响。
