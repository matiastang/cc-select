---
description: "Task list for 002-model-picker-sync（Mode P 会话内真实模型选择与显示）"
---

# Tasks: model-picker-sync（Mode P 会话内真实模型选择与显示）

**Input**: Design documents from `/specs/002-model-picker-sync/`

**Prerequisites**: plan.md ✅ spec.md ✅ research.md ✅ data-model.md ✅ contracts/ ✅ quickstart.md ✅

**Tests**: 包含测试任务——宪法 VIII（TDD，NON-NEGOTIABLE）：每个实现任务前必须有先失败的测试任务（red → green → refactor）。

**Organization**: 按用户故事分组；TDD 顺序；commit 粒度 = 一个功能点一个 commit（宪法 Development Workflow）。

## Format: `[ID] [P?] [Story] Description`

- **[P]**: 可并行（不同文件、无未完成依赖）
- **[Story]**: 所属用户故事（US1=启动即见真实模型 P1，US2=选择生效 P2，US3=热切跟随 P3）
- 每个任务含确切文件路径

---

## Phase 1: Setup（基线与产物入库）

**Purpose**: 确认 001 合入后的绿色基线；002 分析产物入库

- [ ] T001 基线验证：工作分支 dev/tdy 上 `make check` 与 `go test ./internal/...` 全绿（001 fast-forward 合入后的起点确认；有红先修基线，不动功能代码）
- [ ] T002 提交 002 分析产物：暂存的 `docs/requirements.md`（R10）+ `specs/002-model-picker-sync/`（spec/plan/research/data-model/contracts/quickstart/checklists）一个 docs commit（Conventional Commits：`docs(specs): 新增 002 model-picker-sync SDD 产物与 R10 需求`）

---

## Phase 2: Foundational（阻塞前提：ModelPlan 派生）

**Purpose**: 三处消费者（注入/改写/刷新）共用的唯一派生点——显示与路由同源的关键不变量（research D1）

**⚠️ CRITICAL**: 所有用户故事依赖本阶段；未完成前任何故事不得开工

- [ ] T003 失败测试：派生规则全表——`internal/config/modelplan_test.go` 先写 red 测试：四槽派生与 Entries 保序（main→opus→sonnet→haiku）、同 id 多槽去重（首槽位保留）、`$keychain:` 占位跳过、部分槽位为空、四槽全空 → 空 plan（不注入信号）
- [ ] T004 实现使 T003 转绿：`internal/config/modelplan.go` — `ModelPlan{Main,Opus,Sonnet,Haiku,Entries}` / `ModelEntry{ID,Slot}` / `Slot` 类型 + `ModelPlanFromEnv(env map[string]string) ModelPlan`（复用 `config.IsKeychainPlaceholder`）

**Checkpoint**: `go test ./internal/config/...` 绿；ModelPlan 可被三处消费者引用

---

## Phase 3: User Story 1 - 启动会话即见真实模型（Priority: P1）🎯 MVP

**Goal**: `use` 构建 profile 时注入 `modelPicker`（+ Mode P 的 `model` 字段），进入 claude 打开 `/model` 只见当前 provider 真实模型（spec US1；契约 [contracts/profile-settings.md](contracts/profile-settings.md) INV-1~5）

**Independent Test**: quickstart.md 场景 1 + 对照组（官方 provider 无注入）；`jq '.modelPicker' ~/.cc-select/profiles/<id>/settings.json` 断言形状

### Tests for User Story 1（先写，确认 FAIL）

- [ ] T005 [P] [US1] 失败测试·注入形状：`internal/profile/modelpicker_test.go` — mergeSettings 输出含 `modelPicker{replaceBuiltInOptions:true, options:[{model:...}]}`；options 仅含 `model` 键（无 label/description，research D7）；顺序 = ModelPlan.Entries
- [ ] T006 [P] [US1] 失败测试·边界全集：四槽全空/官方 provider → 零注入（键不存在）；Mode P → 额外注入 `model=<主模型>`；Mode B → 无 `model` 注入；既有未知字段（permissions/hooks/`model`）全保留；`availableModels` 存在时追加缺失 id（幂等，research D5）；注入内容无 `$keychain:` 占位（INV-1）

### Implementation for User Story 1

- [ ] T007 [US1] 注入实现：`internal/profile/merge.go` — `mergeSettings` 收尾步注入 `modelPicker` + `availableModels` 幂等追加；`internal/profile/build.go` 的 `syncFull` 同样注入（Mode A，FR-008）；抽出共享 `buildPickerBlock(plan ModelPlan) map[string]any` 供 US3 刷新复用
- [ ] T008 [US1] Mode P 真值源接线：`internal/profile/build.go` `SyncProxy` 签名扩展接收 provider 真值 env（`internal/cli/use.go` 的 `runUseProxy` 调用点传入 `target.Env`），据此派生 ModelPlan 注入 `modelPicker` + `model`（注意：profile env 只有代理 BASE_URL，绝不可用 profile env 派生——research D2）
- [ ] T009 [US1] 旧版 CC 提示（FR-007/D6）：`internal/cli/use.go` + `internal/i18n/locales/{en,zh}.json` — best-effort `claude --version` semver 解析，< 2.1.242 时 `use` 输出一行升级提示；解析失败/无 claude 静默跳过，绝不影响 `use` 主流程

**Checkpoint**: quickstart 场景 1 全流程通过（含官方 provider 对照组）；MVP 可交付

---

## Phase 4: User Story 2 - /model 中选择真实模型并生效（Priority: P2）

**Goal**: 代理改写从「无条件主模型」升级为 R1–R6 映射化规则（spec US2；契约 [contracts/proxy-model-routing.md](contracts/proxy-model-routing.md)）

**Independent Test**: quickstart.md 场景 2 + 回归对照（后台 haiku 任务仍映射 Haiku 槽）；daemon 转发请求体 `model` 断言

### Tests for User Story 2（先写，确认 FAIL）

- [ ] T010 [P] [US2] 失败测试·契约表全量：`internal/router/modelrewrite_test.go` — 按契约示例表逐行：清单内透传（含 `[1m]` 形态精确匹配）/`claude-opus-5`、`opus`、`opus[1m]` → Opus 槽/`haiku` → Haiku 槽/sonnet 槽空 → 回落主模型/未知 id（热切后旧 provider id）→ 主模型/无 `ANTHROPIC_MODEL` → 逐字节透传（v1 回归）；既有 32 MiB/非 JSON 防护用例保持绿

### Implementation for User Story 2

- [ ] T011 [US2] 映射化改写：`internal/router/modelrewrite.go` — `targetModel` 单值返回升级为「清单精确匹配 → opus/sonnet/haiku 子串槽位分类（清单匹配优先于子串，防 provider id 误判）→ 主模型回落 → 主模型为空透传」；槽位数据经 `config.ModelPlanFromEnv(providerEnvFor(providerID))` 与注入同源；每请求派生开销可忽略（四个 map 查找），如需可后续加缓存（YAGNI，先不加）

**Checkpoint**: quickstart 场景 2 通过；US1 注入的选择在 Mode P 下 100% 透传不被改写（SC-003）

---

## Phase 5: User Story 3 - 热切后选择器跟随新服务商（Priority: P3）

**Goal**: `route switch` 成功后刷新**发射 profile** 的 settings.json（`modelPicker` + `model`），会话内打开 `/model` 即见新 provider 清单（spec US3；research D3 正确性核心）

**Independent Test**: quickstart.md 场景 3/4/5（热切跟随 / 终端隔离 / 并发写入不丢字段）

### Tests for User Story 3（先写，确认 FAIL）

- [ ] T012 [P] [US3] 失败测试·刷新原语：`internal/profile/refresh_test.go` — `RefreshPicker`：字段级合并（`model`/其他字段保留）、`modelPicker`+`model` 覆写为新 plan、原子写（临时文件+rename）、目标 JSON 损坏 → 返回错误（调用方决定告警）
- [ ] T013 [P] [US3] 失败测试·CLI 集成：`internal/cli/route_cli_test.go` — `runRouteSwitch` 成功后：`$CLAUDE_CONFIG_DIR` 已设 → 该目录 settings.json 被刷新（发射 profile 语义：切换到 B 时写的是 A 的 profile 目录——research D3）；未设 → 跳过且切换成功；目标文件损坏 → 切换成功 + stderr 告警

### Implementation for User Story 3

- [ ] T014 [US3] 刷新原语：`internal/profile/refresh.go` — `RefreshPicker(settingsPath string, plan config.ModelPlan) error`：读-改-写 + 原子写（沿用 `EnsureRaw` 的 temp+rename 模式）；复用 T007 的 `buildPickerBlock`
- [ ] T015 [US3] 切换接线：`internal/cli/route.go` — `runRouteSwitch` 在 `routes.Set` 成功后：`os.Getenv("CLAUDE_CONFIG_DIR")` → 定位 settings.json（未设跳过）→ `target` 的 `Provider.Env` 派生 ModelPlan → `RefreshPicker`；失败仅 stderr 告警，**路由切换结果不受影响的**（路由表是 Mode P 真值）

**Checkpoint**: quickstart 场景 3/4/5 全部通过；US1/US2 不回退

---

## Phase 6: Polish & Cross-Cutting（文档回流 + 质量门禁）

**Purpose**: 宪法 V/VII 与 research D9 的回流义务；合入前质量闭环

- [ ] T016 [P] 验收回流：`docs/acceptance-tests.md` 新增 AC17（对齐 quickstart 五场景 + 失败路径抽查表；标注自动化覆盖范围：Go 单测/集成对应关系）
- [ ] T017 [P] 事实修正回流：`docs/engineering-decisions.md` §8「claude env 会话内冻结」表述修正（research D9 ②：官方现行文档确认 settings 大多热重载，冻结键仅 model/effortLevel/outputStyle；Mode P 架构依据不受影响）
- [ ] T018 [P] 模式文档回流：`docs/isolation-modes.md` Mode P 节补「模型显示与切换」小节（三故事能力 + model 不热重载的已知限制）+ README「已知限制」同步（en/zh/ja 语言切换器路径核对）
- [ ] T019 全量门禁：`make check` + `make test` + `make integration` 全绿
- [ ] T020 循环 code review（宪法/开发基本要求）：静态检查 + 评审，修复所有中等严重及以上问题并复评至清零

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup（P1）**：无依赖；T002 建议在一切代码任务前完成（产物入库，diff 干净）
- **Foundational（P2）**：依赖 Setup；**阻塞全部用户故事**（ModelPlan 是三者共同数据源）
- **US1（P3）**：依赖 T004；MVP
- **US2（P4）**：依赖 T004；**不依赖 US1 完成**（改写只认 ModelPlan + env）——但端到端验收（选择生效的完整体验）需 US1 的注入先就位，建议按序执行
- **US3（P5）**：依赖 T004 + T007（复用 `buildPickerBlock`）；建议 US1/US2 后执行（quickstart 场景 3 需场景 1/2 的能力才有意义）
- **Polish（P6）**：依赖 US1–US3 全部完成

### Within Each Story

- 测试任务先写并确认 FAIL（宪法 VIII）→ 实现转绿 → 重构
- T007 在 T005/T006 之后；T008 依赖 T007；T011 依赖 T010；T014 依赖 T007；T015 依赖 T013/T014

### Parallel Opportunities

- T005 ∥ T006（不同测试文件/关注点）
- T010 独立（不同包）
- T012 ∥ T013（refresh 原语测试 ∥ CLI 集成测试）
- T016 ∥ T017 ∥ T018（三个文档文件互不重叠）
- 单线开发时整体按 T001→T020 顺序执行即可；[P] 标记仅在有并行执行者时有意义

---

## Parallel Example: User Story 1（TDD 波次）

```text
第 1 波（并行）：T005「注入形状失败测试」 + T006「边界全集失败测试」
第 2 波（串行）：T007 实现 → T008 Mode P 接线 → T009 版本提示
验证：quickstart 场景 1（含官方 provider 对照组）
```

---

## Implementation Strategy

### MVP First（US1 Only）

1. T001–T002（基线 + 产物入库）→ 2. T003–T004（ModelPlan）→ 3. T005–T009（US1）→ **STOP**：quickstart 场景 1 验证 → 可交付「启动即见真实模型」

### Incremental Delivery（推荐顺序）

1. Foundational → 2. US1（看到）→ 3. US2（选了生效）→ 4. US3（热切跟随）→ 5. Polish 回流
   每个故事独立可验收，不回退前序故事；每完成一个故事按 commit 粒度分批提交（测试与实现同故事内按 TDD 节奏提交，功能点间不混 commit）

### Commit 粒度提示（宪法）

- T004（ModelPlan）单独一个 commit；T007+T008（注入+Mode P 接线）可按「feat(profile): 注入 modelPicker」与「feat(profile): Mode P 真值源接线」拆两个；T011「refactor(router)→feat(router) 映射化改写」一个；T014+T015「feat(route): 热切刷新选择器」一个；T016–T018 各一个 docs commit；T009 可并入 US1 或独立 `feat(cli)` commit

---

## Notes

- [P] = 不同文件、无未完成依赖
- 所有 settings.json 写入必须原子（temp+rename）且 0600（沿用现状权限）
- 注入/刷新全程零密钥（INV-1）：ModelPlan 派生显式排除 `$keychain:` 占位
- T015 的 `$CLAUDE_CONFIG_DIR` 继承机制 = 001 研究 D6 已确立的 `CC_SELECT_TID` 同信道；若集成测试暴露三平台差异，回到 research D3 补充决策
- 任何任务发现 spec/plan 与代码现实冲突：停手，回到 plan 修正（宪法 VII），不得 silently 改语义
