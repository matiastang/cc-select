# Tasks: 代码质量门禁（v0.1.3）

**Input**: Design documents from `/specs/005-quality-gates/`

**Prerequisites**: plan.md, spec.md, research.md（阈值与工具定案）, data-model.md（基线 schema）, contracts/quality-gates.md（接口）, quickstart.md（注入式验收）

**Tests**: qualitygate 是本特性唯一的新 Go 代码，严格 TDD（宪法 VIII）；配置类任务以 quickstart 注入实验验收。

**Organization**: 按 US 分组；每个任务 = 一个功能点 = 一个 commit（宪法 commit 粒度）。

## Format: `[ID] [P?] [Story] Description`

- **[P]**: 可并行（不同文件、无依赖）
- **[Story]**: 所属用户故事
- 描述含确切文件路径

## Path Conventions

- 自研工具：`tools/qualitygate/`（Go，独立 main 包）
- 配置：仓库根（`.golangci.yml`、`quality-baseline.json`、`Makefile`、`lefthook.yml`、`.github/workflows/ci.yml`）
- 前端：`internal/frontend/`（eslint.config.js、vitest.config.ts、knip.json、package.json）

---

## Phase 1: Setup（工具链就位，不改任何门禁行为）

**Purpose**: 版本锁定（FR-017）与工具可用性；全部任务落地后既有 CI/check 行为不变。

- [x] T001 [P] go.mod 增 `tool` 指令：`github.com/golangci/golangci-lint/v2/cmd/golangci-lint` v2.15.0 + `golang.org/x/vuln/cmd/govulncheck` v1.9.0；`go mod tidy` 后 `go build ./internal/... .` 与 `go test ./internal/...` 不受影响；`go tool golangci-lint version` 输出 v2.15.0。commit: `chore(quality): 锁定 golangci-lint 与 govulncheck 工具依赖`
- [x] T002 [P] 新增 `.golangci.yml`（data-model Entity 2：default:none 白名单 cyclop/funlen/lll/dupl/unused + 阈值 15/100·60/160/150 + 排除 node_modules 与 _test.go 豁免）；此任务只落配置不接线，`make check` 行为不变。commit: `chore(quality): 新增 golangci-lint v2 白名单配置`
- [x] T003 [P] `internal/frontend/package.json` 增 devDependencies：jscpd@5.4.2、knip@6.44.1（锁版本）+ scripts（`quality:dup`/`quality:dead` 输出 JSON 供棘轮消费）；`npm install` 更新 lock。commit: `chore(quality): 前端新增 jscpd 与 knip 锁版依赖`

**Checkpoint**: `go tool golangci-lint run`、`npx --no jscpd/knip` 三个工具本地可运行（手动验证即可，未接入门禁）。

---

## Phase 2: Foundational（基线文件 + qualitygate 骨架，阻塞所有 US）

**Purpose**: 棘轮机制的载体与引擎；⚠️ 后续所有任务依赖本阶段。

- [x] T004 **TDD-red→green**：`tools/qualitygate/` 新建 Go 模块内包（`package main`），先写 `baseline_test.go`：TestLoadBaseline（合法 JSON 解析、version 校验、缺字段报错）、TestFindingMatches（file+line+rule 精确、message 前缀匹配、line=0 整文件匹配）。实现 `baseline.go`（data-model Entity 1 结构体）。commit: `feat(quality): qualitygate 基线加载与匹配（TDD）`
- [x] T005 依据 research §0 实测数据落 `quality-baseline.json` 初版（data-model Entity 1 格式）：findings.golangci 7 条（cyclop×3、dupl×1 对即 powershell.go 单条、lll×1、unused×2）+ counters（jscpd-go:10、jscpd-frontend:5、knip-frontend:15）+ coverage（go 76 / frontend 79）+ fileLines.go-max 1000；运行 `go run ./tools/qualitygate filter` 自测解析。commit: `chore(quality): 落地质量基线（存量实测快照）`

**Checkpoint**: 基线可加载、可匹配；三个 US 的实现只剩"工具→接线"。

---

## Phase 3: User Story 1 — 提交拦截：超限代码进不了仓库（P1）🎯 MVP

**Goal**: 本地 `git commit` 拦截规模/复杂度违规，报告含位置与实测/阈值（FR-003~007）。

**Independent Test**: 注入超限函数/文件 → commit 被拦截；还原后通过（quickstart 场景 1）。

### Implementation for User Story 1

- [x] T006 **TDD-red→green** [US1] `tools/qualitygate/filelines_test.go`：TestFileLines 超限检出（构造 tmp 目录：一个 1200 行 .go、一个 900 行、一个 _test.go 超长但豁免、排除 glob 命中目录）；实现 `filelines.go`（walk + 计数，跳过 `_test.go` 与排除项，输出 file:lines>max 清单）。commit: `feat(quality): Go 文件行数检查子命令（TDD）`
- [x] T007 [US1] `Makefile` 增 `quality-go`（`go tool golangci-lint run | go run ./tools/qualitygate filter --tool golangci` 管道 + `qualitygate filelines`）与 `quality-go-new`（同上，golangci 带 `--new-from-rev HEAD`）；验证：全量过基线后 exit 0，注入超限函数后 exit 1 且报告含位置。commit: `feat(quality): Go 门禁 make 目标（golangci+基线过滤+文件行数）`
- [x] T008 [US1] `internal/frontend/eslint.config.js` 增 5 条规则（max-lines 500 / max-lines-per-function 150 / complexity 15 / max-depth 4 / max-params 4，data-model Entity 3）+ test 文件 override 关闭规模类规则；`npx eslint src` 剩余违规 = research §0.2 的 12 条。commit: `feat(quality): ESLint 规模与复杂度规则（含 test 豁免）`
- [x] T009 [US1] **TDD-red→green** `tools/qualitygate` 增 `filter --tool eslint`（读 ESLint JSON stdin，file 行号 rule message 对齐基线 findings.eslint）；Makefile 增 `quality-frontend`（eslint 全量 JSON → filter）。commit: `feat(quality): eslint 基线过滤接入（TDD）`
- [x] T010 [US1] `lefthook.yml` pre-commit 增 `quality-go`（glob `**/*.go` → `make quality-go-new`）与 `quality-frontend`（glob `internal/frontend/src/**/*.{ts,tsx}` → `make quality-frontend-staged`）；现有 hook 一字不动；staged 版本 eslint 增量实现（eslint 仅 staged 文件列表入参）。commit: `feat(quality): pre-commit 增量质量检查接线`
- [x] T011 [US1] 验证 quickstart 场景 1 全部 4 个验收场景（注入超长函数/超长文件/高复杂度/合规快速通过并计时 ≤10s）；`git commit --no-verify` 后 US2 前 CI 不误伤（本地配置合法）。commit: 无代码变更，验证记录进 tasks 复选框

**Checkpoint**: US1 独立可用——注入违规提交必被拦截，合规提交无感（秒级）。

---

## Phase 4: User Story 2 — CI 权威门禁（P1）

**Goal**: CI 全量校验 + 同一基线，绕过本地也进不了主干（FR-001/002/009）。

**Independent Test**: 推送违规分支 → lint job 红；修复 → 绿（quickstart 场景 2）。

### Implementation for User Story 2

- [x] T012 [US2] `.github/workflows/ci.yml` lint job 增步骤：`make quality`（新聚合目标：quality-go + quality-frontend + 后续 dup/dead）——golangci 首次在 CI 冷启动编译 tool 依赖的时长评估，必要时用 actions/cache 缓存 `~/go/pkg/mod`；`make check` 同步增 `quality`。commit: `feat(quality): CI lint job 接入全量质量门禁`
- [x] T013 [US2] 验证：本地模拟 CI 路径（`make quality` + `make check` 全绿）；两端结论一致（SC-003：同一基线文件、同一工具版本）。commit: 无代码变更

**Checkpoint**: US2 独立可用——CI 是权威门禁，与本地同口径。

---

## Phase 5: User Story 3 — 增量与棘轮（P1）

**Goal**: 存量零清算（基线豁免）+ 棘轮不可逆（FR-008/010）。

**Independent Test**: quickstart 场景 3 的 5 步实验。

### Implementation for User Story 3

- [x] T014 **TDD-red→green** [US3] `tools/qualitygate/ratchet_test.go`：TestRatchetCounter（现值>基线 fail / <基线 pass+提示 / =通过）、TestRatchetCoverage（低于 min fail）；实现 `ratchet.go` 子命令（读 jscpd/knip JSON 与 coverage total）。commit: `feat(quality): 计数与覆盖率棘轮子命令（TDD）`
- [x] T015 [US3] `tools/qualitygate` 增棘轮不可逆自检：`filter` 输出中若发现"基线条目在本次工具输出中不存在"→ 提示删除条目（棘轮向下的唯一合法路径）；验证场景 3 步骤 3/4/5。commit: `feat(quality): 基线失效条目回收提示`
- [x] T016 [US3] 端到端验证 US3 全部 4 个验收场景（存量不阻塞/增量拦截/基线 diff 可见/棘轮不可逆），记录进 quickstart 勾选。commit: 无代码变更

**Checkpoint**: 零清算上线达成（SC-002/005）。

---

## Phase 6: User Story 4 — 覆盖率阈值（P2）

**Goal**: 覆盖率跌破阈值 CI 失败并报数值（FR-011）；顺带修复 vitest coverage.include 配置腐化。

- [x] T017 [P] [US4] `internal/frontend/vitest.config.ts`：修复 coverage.include → `src/**/*.{ts,tsx}`（exclude 不变）+ 增 thresholds（79/80/77/79，research §7）；`npm run coverage` 实测真实覆盖率且阈值生效。commit: `fix(quality): 修复 vitest 覆盖率 include 收窄并加阈值`
- [x] T018 [US4] **TDD-red→green** `tools/qualitygate` `coverage` 子命令（读 `go tool cover -func` total，≥min 通过）；Makefile 增 `quality-coverage`（Go 棘轮 + `npm run coverage`）；ci.yml test job Linux runner 增该步骤。commit: `feat(quality): Go 覆盖率棘轮接入（TDD）`

**Checkpoint**: US4 独立可用——注入无测试代码 → CI 覆盖率门禁红。

---

## Phase 7: User Story 5 — 重复代码检测（P2）

**Goal**: jscpd 克隆计数棘轮（FR-012）。

- [x] T019 [US5] Makefile 增 `quality-dup`：jscpd（Go：min-tokens 60 排除 `**/*_test.go`/node_modules；前端：min-tokens 50 排除 `**/*.test.*`）JSON 输出 → `qualitygate ratchet --metric jscpd-go|jscpd-frontend`；接入 `make quality` 与 ci.yml。commit: `feat(quality): 重复代码计数棘轮接入`
- [x] T020 [US5] 验证：注入跨文件复制粘贴块 → 计数上升 → FAIL；误报抽查（阈值 50/60 tokens 下惯用样板不触发，research §0 实测佐证）。commit: 无代码变更

---

## Phase 8: User Story 6 — 依赖漏洞扫描（P2）

**Goal**: 可触达高危漏洞阻塞 CI（FR-013）。

- [x] T021 [US6] Makefile 增 `quality-vuln`：`go tool govulncheck ./...`（调用图口径，exit 3=可触达漏洞→失败；仅 imported→警告放行）+ `npm audit --audit-level=high --omit=dev`（research §8 已知近似记录在案）；接入 ci.yml lint job。commit: `feat(quality): 依赖漏洞扫描接入 CI`
- [x] T022 [US6] 验证：govulncheck 当前依赖树结论记录（预期干净或未触达警告）；npm audit 当前生产依赖结论记录；注入实验（git stash 一个带已知漏洞的旧版本依赖）可选项跳过亦可——记录原因。commit: 无代码变更

---

## Phase 9: User Story 7 — 死代码检测（P3）

**Goal**: 未引用导出/依赖可见、增量清零（FR-014）。

- [x] T023 [US7] `internal/frontend/knip.json` 配置（入口/插件按 vite+react+vitest 实际核实；豁免显式 `ignore` 带注释）+ Makefile `quality-dead`（knip JSON → ratchet knip-frontend）；接入 `make quality` 与 ci.yml；golangci unused 已含于 quality-go（基线 2 条）。commit: `feat(quality): knip 死代码计数棘轮接入`
- [x] T024 [US7] 验证：注入无引用导出 → knip 计数 16 → FAIL；确认豁免机制（knip ignore / `//nolint:unused`）在文档可查。commit: 无代码变更

---

## Phase 10: Polish & Cross-Cutting

**Purpose**: 文档回流（宪法 V/VII）与整体验收。

- [x] T025 [P] 新增 `docs/quality-gates.md` 门禁维护手册（FR-018）：每类检查的阈值/语义/本地运行/单点豁免（nolint、eslint-disable、knip ignore）/修基线流程（棘轮规则 + PR 评审要求）/工具升级流程（FR-017）。commit: `docs(quality): 门禁维护手册`
- [x] T026 [P] `docs/acceptance-tests.md` 增 AC20（质量门禁验收用例，映射 spec 7 US + SC-001~006）；`docs/README.md` 文档索引更新。commit: `docs(quality): AC20 验收用例与文档索引`
- [x] T027 全量回归：`make check` 全绿（含新 quality 目标）+ Go 单测/集成 + vitest 118 + Playwright 19 + `make quality` 全绿；quickstart 场景 1–5 全部走查。commit: 无代码变更
- [x] T028 循环 code review（静态 + 人工清单）：修复所有中等严重及以上问题并复评直至清零（宪法工作流）；review 范围含 specs/005 全部产物与实现 commit 链。commit: review 修复（如有）
- [x] T029 回流收尾：`docs/requirements.md` v0.1.3 勾选实现状态 + 变更记录行；constitution 若 Technology Constraints 需补工具链事实（golangci-lint v2.15.0 tool 指令等）则按 PATCH 修订。commit: `docs(quality): v0.1.3 验收回流`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1（Setup）**: 无依赖，三个 [P] 任务可并行
- **Phase 2（Foundational）**: 依赖 T001（go tool 可用性无关 qualitygate 编译，但 filter 自测需要）；T004 → T005 串行
- **Phase 3–5（US1/US2/US3）**: 依赖 Phase 2；US1（T006–T011）→ US2（T012–T013）→ US3（T014–T016）按序——US2/US3 的接线复用 US1 的 make 目标
- **Phase 6–9（US4–US7）**: 依赖 Phase 5（棘轮子命令 T014 是 US4 覆盖率棘轮、US5 jscpd 棘轮、US7 knip 棘轮的前置）；US4–US7 之间可并行（不同文件/工具）
- **Phase 10**: 依赖全部 US 完成

### User Story Dependencies

- US1（提交拦截）是核心骨架：make 目标 T007/T009 被 US2 的 `make quality` 与 US3 的棘轮消费
- US4/US5/US7 共享 T014 的 ratchet 引擎，但各自独立可测
- US6（漏洞）完全独立（纯 CI 步骤）

### Parallel Opportunities

- T001/T002/T003（Phase 1 全并行）
- T017 与 US5/US6/US7 任意任务
- T025/T026（文档，Phase 10 内并行）

---

## Implementation Strategy

- **MVP = Phase 1–3（US1）**: 工具链 → 基线 → 提交拦截。US1 完成即产生核心价值（本地拦截）
- **增量交付**: +US2（CI 权威）→ +US3（零清算与棘轮完整）→ +US4–7（扩展门禁逐个叠加）
- **TDD 边界**: T004/T006/T009/T014/T018 是代码任务（red→green→refactor）；其余为配置/接线/验证任务（quickstart 注入实验验收）
- **每任务一 commit**；验证类任务（无代码）复选框勾选并记录证据于 tasks.md 注释

## Notes

- 阈值与基线数字的唯一来源是 research.md §0 实测（2026-11-06），implement 时若实测复测值漂移，以复测值更新基线并回注 research
- golangci-lint v2 配置语义（default:none 必需、enable-only 不生效）是实测结论（research §1），遇到版本行为差异以实测为准
- npm audit 的"可触达"近似（research §8/§11-R4）在 T026 文档化，不算偏差未闭环
