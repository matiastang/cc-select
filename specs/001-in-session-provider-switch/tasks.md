---
description: "Task list: 会话内切换 provider（R9 / v0.0.6）"
---

# Tasks: 会话内切换 provider（In-Session Provider Switch）

**Input**: Design documents from `/specs/001-in-session-provider-switch/`

**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅, quickstart.md ✅

**Tests**: 本项目宪法 VIII 强制 TDD（NON-NEGOTIABLE）——每个实现任务均为「先写失败测试（red）→ 实现（green）→ 重构（refactor）」，red 与 green 在**同一 commit** 内完成（宪法 Development Workflow）。

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- Go 后端/CLI/daemon：`internal/<pkg>/`；前端：`internal/frontend/src/`；e2e：`internal/frontend/e2e/`
- 契约真值：[contracts/cli.md](./contracts/cli.md)、[contracts/router-http.md](./contracts/router-http.md)、[contracts/web-api.md](./contracts/web-api.md)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: 分支与基线准备

- [x] T001 建特性分支 `001-in-session-provider-switch`（基线按 PR 计划取 main 或 dev/tdy），确认 `make check` 基线全绿

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: 三个共享基石：模式枚举、路由表存储、守卫式发射。US1/US3/US4 都依赖。

- [x] T002 [P] TDD：`prefs.ModeProxy`（`"proxy"`）——先在 `internal/prefs/prefs_test.go` 写失败用例（`Valid()` 接受 proxy、`ResolveMode` 三级解析兼容、`DefaultMode` 仍为 settings-only），再改 `internal/prefs/prefs.go`、`internal/prefs/resolve.go`；同步扩展 `internal/cli/mode.go` 值域校验与帮助文案（研究 D4）
- [x] T003 [P] TDD：`internal/routes` 路由表包——先写 `internal/routes/routes_test.go` 失败用例（Load/Save/List/Set/Switch/Prune、schema `{"version":1,"routes":[...]}`、原子写 0600、tid 主键 `^ccs-[0-9a-f]{32}$`、updatedAt RFC3339 刷新），再实现 `internal/routes/routes.go`（纯存储，不依赖 config 包；provider 存在性校验留在 CLI/Web 层，见 data-model §2）
- [x] T004 [P] TDD：`shell.Change` 增 `OpSetIfUnset`——先在 `internal/shell/shell_test.go`、`internal/shell/powershell_test.go` 写失败用例（zsh/bash 渲染 `if [ -z "${VAR:-}" ]; then export VAR='v'; fi`；PowerShell 渲染 `if (-not $env:VAR) { $env:VAR = 'v' }`），再改 `internal/shell/shell.go`、`internal/shell/zsh.go`、`internal/shell/powershell.go`
- [x] T005 [P] TDD：TID 生成——先在 `internal/routes/routes_test.go` 补失败用例（`NewTID()` 前缀 `ccs-`+32 hex、唯一性），再实现于 `internal/routes/routes.go`（crypto/rand，研究 D6）

**Checkpoint**: 基石就绪，US1/US3/US4 可并行开工。

---

## Phase 3: User Story 1 - 限额救急：会话内热切 (Priority: P1) 🎯 MVP

**Goal**: Mode P 全链路：`ccs use`（proxy）→ daemon 转发 → 会话内 `route switch` 热切，不重启会话、上下文延续、终端间隔离（FR-001~004/006/007/010/011）。

**Independent Test**: quickstart.md §2（真机）+ §4（自动化）；SC-001/002/006。

### Implementation for User Story 1（每任务内含 TDD red→green）

- [x] T006 [US1] TDD：profile Mode P 构造——先在 `internal/profile/build_test.go` 写失败用例（settings.json env 仅 `{ANTHROPIC_BASE_URL: "http://<addr>"}`、复用 Mode B 共享链接白名单、不含 AUTH_TOKEN/任何密钥、官方 provider 仍 no-op），再改 `internal/profile/build.go`（data-model §5，研究 D5）
- [x] T007 [US1] TDD：switcher proxy 发射——先在 `internal/switcher/switcher_test.go` 写失败用例（发射顺序：`CC_SELECT_TID` 守卫式 OpSetIfUnset（NewTID 值）→ `ANTHROPIC_AUTH_TOKEN="$CC_SELECT_TID"` 引用式 → `CLAUDE_CONFIG_DIR` → `CC_SELECT_ACTIVE`；官方 provider 回退既有发射），再改 `internal/switcher/switcher.go`（contracts/cli.md §3）
- [x] T008 [P] [US1] TDD：daemon 状态与自愈——先写 `internal/router/state_test.go` 失败用例（router.json {addr,pid,startedAt,version}；addr 优先级 状态文件>CC_SELECT_PROXY_ADDR>默认 127.0.0.1:48270；ensureDaemon：healthz 健康复用/死亡重启/版本不匹配换新，用 httptest 模拟），再实现 `internal/router/state.go`（研究 D7/D11）
- [x] T009 [P] [US1] TDD：daemon 服务面——先写 `internal/router/server_test.go` 失败用例（仅 loopback 监听；Bearer tid 未知/缺失 → 401 且 body 含 `ccs use` 指引；`/healthz` 返回 {status,version}；stop 侧信道幂等），再实现 `internal/router/server.go`（contracts/router-http.md §1）
- [x] T010 [P] [US1] TDD：转发内核——先写 `internal/router/forward_test.go` 失败用例（剥离入站 Authorization；ANTHROPIC_AUTH_TOKEN→`Authorization: Bearer`、ANTHROPIC_API_KEY→`x-api-key`；SSE FlushInterval 立即；上游非 2xx 原样透传；连接失败 502+明确消息），再实现 `internal/router/forward.go`（contracts/router-http.md §2）
- [x] T011 [P] [US1] TDD：model 改写——先写 `internal/router/modelrewrite_test.go` 失败用例（JSON body 且 provider 定义 ANTHROPIC_MODEL → 替换 model 字段并更新 Content-Length；未定义/非 JSON 透传），再实现 `internal/router/modelrewrite.go`（研究 D9/L3）
- [x] T012 [P] [US1] TDD：keychain 占位解析——先写 `internal/router/resolve_test.go` 失败用例（`$keychain:` 占位→真值、内存缓存命中不重复取、fake SecretStore 注入、取失败可诊断错误），再实现 `internal/router/resolve.go`（复用 `internal/secrets.SecretStore`，研究 D8）
- [x] T013 [US1] 集成测试（`-tags integration`）：`internal/router/integration_test.go`——httptest 起 fake upstream（记录 model/Authorization/path）+ 真 daemon 随机端口 + 伪 claude 客户端（Bearer tid）：切换后**下一笔**走新 provider、在途请求按原 provider 完成、model 改写生效、401 指引（研究 D13）
- [ ] T014 [P] [US1] TDD：`route` 命令族——先在 `internal/cli/cli_test.go` 写失败用例（switch：tid 取 `CC_SELECT_TID` 或 `--tid`、provider 校验（v1 禁 `claude-official`）、输出 `<old> → <new>`、失败不落盘；list 短码展示；status 含 router 存活；prune `--older-than` 默认 168h），再实现 `internal/cli/route.go`（contracts/cli.md §1）
- [ ] T015 [US1] TDD：`router` 命令族——先写失败用例再实现 `internal/cli/router.go`（ensure 幂等 / serve `--foreground` / stop；detached spawn：Unix `setsid`、Windows `DETACHED_PROCESS|CREATE_NEW_PROCESS_GROUP`；serve 沿用状态文件 addr）（contracts/cli.md §2）
- [ ] T016 [US1] TDD：`use` 集成 proxy 模式——先在 `internal/cli/cli_test.go` 写失败用例（mode 解析含 proxy → router ensure（失败即中止+恢复指引）→ `routes.Set` → `profile.Sync`(Mode P) → 发射；官方目标回退既有路径并提示），再改 `internal/cli/use.go`（contracts/cli.md §3，FR-011）
- [ ] T017 [US1] i18n 补全：`internal/i18n/`（en/zh）——T014/T015/T016 全部新增用户可见文案的 key 与译文（宪法 IV：i18n 约定）

**Checkpoint**: quickstart.md §2 全流程手工通过（会话内热切 + 上下文延续 + 不重启）。

---

## Phase 4: User Story 2 - 降级路径：退出→切换→继续 (Priority: P2，即 R9-P0，可先行)

**Goal**: 三步工作流文档化 + 验收覆盖（FR-005/SC-004）。零代码依赖，不依赖 Phase 2/3。

**Independent Test**: quickstart.md §1（暗号法）。

- [x] T018 [P] [US2] `docs/acceptance-tests.md` 增「跨 provider 续会话」用例：Mode B 三步工作流（退出→`ccs use B`→`claude --continue`）上下文延续断言；Mode A 显式标注不适用及原因
- [x] T019 [P] [US2] 限额救急三步工作流文档化：根 `README.md` + `docs/language/README.zh.md`（语言切换器相对路径同步，宪法 IV）+ `docs/isolation-modes.md`（Mode B 共享 `projects/` 是该工作流的物质基础）；不新增组合命令（研究 D3）

---

## Phase 5: User Story 3 - 知道现在用的是谁 (Priority: P3)

**Goal**: `current` 在 Mode P 下以路由表为真值（FR-008/SC 一致性）。

**Independent Test**: quickstart.md §2 中切换后 `cc-select current` 显示新 provider。

- [ ] T020 [US3] TDD：`current` 增量语义——先在 `internal/cli/cli_test.go` 写失败用例（env 有 `CC_SELECT_TID` 且 routes 有条目 → 输出路由真值；无 TID/无条目 → 既有行为读 `CC_SELECT_ACTIVE` 完全不变），再改 `internal/cli/current.go`（contracts/cli.md §4）

**Checkpoint**: 切换反馈、`route status`、`current` 三处一致（US3 验收场景 1/2）。

---

## Phase 6: User Story 4 - 密钥不再明文躺盘 (Priority: P4)

**Goal**: 启用 Mode P 触发密钥迁移 keychain（FR-009）。

**Independent Test**: quickstart.md §5（grep 核对占位符与 profile 无密钥）。

- [ ] T021 [US4] TDD：密钥迁移——先写失败用例（设 mode=proxy 时：敏感值 `ANTHROPIC_AUTH_TOKEN`/`ANTHROPIC_API_KEY` 写 keychain service `cc-select:<id>:<var>` + `providers.json` 原值换 `$keychain:` 占位；幂等（已是占位跳过）；失败返回明细不中断；离开 proxy **不**回迁），实现于 `internal/secrets/migrate.go` + `internal/secrets/migrate_test.go`，调用点接 `internal/cli/mode.go`（研究 D8）
- [ ] T022 [US4] TDD：Web 模式端点——先在 `internal/web/api_test.go` 写失败用例（PUT `/api/v1/mode` 值域含 proxy 且触发迁移、响应 `{mode,migrated,failed}`；GET 回显），再改 `internal/web/api.go`（contracts/web-api.md §1）

**Checkpoint**: 未启用 Mode P 时全链路行为零变化（SC-005）。

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Web GUI、宪法修订、文档回流、整体质量门禁。

- [ ] T023 [P] TDD：Web 路由/守护端点——先在 `internal/web/api_test.go` 写失败用例（GET `/api/v1/routes` 返回 `{router:{running,addr,version},routes:[...]}`（tid 短码）；DELETE `/api/v1/routes?olderThan=` 返回 `{pruned:N}`；POST `/api/v1/router/ensure` 成功/503 语义），再改 `internal/web/api.go`（contracts/web-api.md §2–4）
- [ ] T024 [P] 前端增量：`internal/frontend/src/`（模式选择器增 proxy 选项+三模式说明、「活跃路由」面板（daemon 徽标/路由表/prune/ensure 按钮）；TS `strict` 禁 `any`；react-i18next en/zh）
- [ ] T025 [P] e2e：`internal/frontend/e2e/` 增用例（模式选择含 proxy 可切换、路由面板渲染/ensure 按钮可点）
- [ ] T026 [P] SC-003 强化：`internal/router/crossswitch_test.go`（`-tags integration`）——双 TID 交叉切换各 ≥10 次，断言 fake upstream 收到的每笔请求归属与各自路由表当前值一致、串扰为 0
- [ ] T027 宪法修订 1.1.0 → 1.2.0：`.specify/memory/constitution.md` 原则 II 增 Mode P 豁免条款（env 仅承载恒定身份；`current` 在 TID 存在时以路由表为 per-terminal 真值）+ 文件头 Sync Impact Report（来源 docs/requirements.md R9）（研究 D12）
- [ ] T028 [P] docs 回流（宪法 VII）：`docs/isolation-modes.md` 增 Mode P 章节、`docs/engineering-decisions.md` 增「身份/路由分离」§、`docs/acceptance-tests.md` 增 P1 用例（热切/隔离/自愈/401）、README 各语言快速上手
- [ ] T029 循环 code review：静态检查（`make check`）+ 代码评审，修复所有**中等严重及以上**问题并复评直至清零（宪法 Development Workflow）
- [ ] T030 quickstart.md 全场景人工验证：§1 P0 暗号法 / §2 热切 / §3 隔离+失败+自愈 / §4 自动化 / §5 安全 grep

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: 无依赖，立即可做
- **Foundational (Phase 2)**: 依赖 Phase 1 —— **阻塞 US1/US3/US4**
- **US2 (Phase 4)**: **无任何依赖**（纯文档），可任意提前（R9-P0 先行交付正是此意）
- **US1 (Phase 3)**: 依赖 Phase 2（T006/T007 需 T004/T005；T013–T016 需 T003/T008/T009）
- **US3 (Phase 5)**: 依赖 T003（routes）；可观测价值依赖 US1 的 route switch
- **US4 (Phase 6)**: 依赖 T002（prefs）；语义上在 US1 可用后验收
- **Polish (Phase 7)**: 依赖全部故事完成（T024/T025 依赖 T023；T026 依赖 T013）

### User Story Dependencies

- **US1 (P1)**: Phase 2 后即可开始，不依赖其他故事
- **US2 (P2)**: 完全独立（建议最先做，兑现 R9-P0）
- **US3 (P3)**: 依赖 Foundational；与 US1 并行开发、US1 后验收
- **US4 (P4)**: 依赖 Foundational；与 US1 并行开发

### Within Each User Story

- 每个任务内部：失败测试（red）→ 实现（green）→ 重构，**同一 commit**
- 一任务一 commit（宪法 commit 粒度）；US2 的 T018/T019 亦各一 commit
- 组件顺序：存储/发射（T006/T007）→ daemon（T008–T012）→ 集成（T013）→ CLI（T014–T016）→ 文案（T017）

### Parallel Opportunities

- Phase 2：T002/T003/T004/T005 全并行（不同包）
- US1 内：T008/T009/T010/T011/T012/T014 相互独立可并行（不同文件，测试用 fake 协作）
- 任意时刻：T018/T019（US2 文档）可与一切并行
- US4 的 T021 与 US1 的 daemon 系并行

---

## Parallel Example: User Story 1

```bash
# Foundational 完成后，daemon 五件套并行开工（不同文件、fake 协作）：
Task T008: internal/router/state.go
Task T009: internal/router/server.go
Task T010: internal/router/forward.go
Task T011: internal/router/modelrewrite.go
Task T012: internal/router/resolve.go

# 汇合后顺序做：T013 集成 → T015 → T016 → T017
```

---

## Implementation Strategy

### MVP First

1. Phase 1 + Phase 2（基石）
2. **US2（T018/T019）随手先做**——零成本兑现 R9-P0，独立可发布
3. Phase 3 US1（核心 MVP：会话内热切全链路）
4. **STOP and VALIDATE**：quickstart §2 手工 + §4 自动化
5. US3 → US4 → Phase 7

### Incremental Delivery

1. US2 = v0.0.6 的 P0 增量（可独立合入）
2. US1+US3 = P1 核心价值（热切 + 状态一致）
3. US4 = 安全收敛
4. Phase 7 = GUI/宪法/docs 回流/质量门禁，收口 v0.0.6

---

## Notes

- [P] = 不同文件、无未完成依赖
- [Story] 标签映射 spec.md 用户故事，可追溯
- 每故事独立可测：US1→quickstart §2/§4；US2→§1；US3→§2 current 一致性；US4→§5
- 实现前必读：contracts/ 三份契约 + research.md 决策编号（任务描述中的 D# 即其引用）
- 验证测试先失败再实现（宪法 VIII）；提交前 `make check`（宪法 V）
- 完成后：spec 目录冻结为过程记录，结论按 T028 回流 docs/（宪法 VII）
