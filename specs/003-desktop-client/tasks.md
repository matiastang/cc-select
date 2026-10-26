---
description: "Task list for feature implementation"
---

# Tasks: Wails v3 桌面客户端（desktop-client）

**Input**: Design documents from `/specs/003-desktop-client/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: **强制包含**（宪法原则 VIII：TDD 先行——每个测试任务先写并确认 FAIL，再实现至 GREEN）。项目开发基本要求另规定：一个功能点一个 commit。

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

**分支约定**：实现自 `dev/tdy` 切出 `003-desktop-client` 特性分支（宪法 Development Workflow）。

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g. US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- 本特性路径：`desktop/`（桌面端入口）、`internal/web/`、`internal/prefs/`、`internal/frontend/src/`、`Makefile`、`.github/workflows/`（见 plan.md Project Structure）

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: 桌面构建目标初始化

- [x] T001 在 go.mod 引入并锁定 `github.com/wailsapp/wails/v3 v3.0.0-beta.34`（`go get`+`go mod tidy`，确认根 `main.go` CLI 构建不受影响）
- [x] T002 [P] 创建 `desktop/` 包骨架（`package main` 占位 main.go、`assets/` 目录与 `.gitignore`（忽略构建期拷入的 CLI 二进制））per plan.md
- [x] T003 [P] Makefile 新增 `desktop-build` / `desktop-package` / `desktop-clean` 目标（依赖链：frontend → CLI → 拷贝 `bin/cc-select-*` 至 `desktop/assets/cli/` → `wails3 build`；不污染既有 `build`）
- [x] T004 [P] 配置 `desktop/` 的 wails3 构建 Task 文件与图标资源（app icon / tray icon 占位）

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: 纯 Go、无 Wails 依赖的可复用基座（全部可单测）

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T005 TDD：在 `internal/web/api_test.go` 先写导出包装测试（`Handler()` 能服务 `/api/v1/providers` 列表、`AssetsFS()` 非空），FAIL 后在 `internal/web/api.go`/`embed.go` 实现 `Handler() http.Handler` 与 `AssetsFS() fs.FS` 导出包装（行为零变化，`ccs gui` 复用同一实现）
- [x] T006 [P] TDD：在 `internal/prefs/prefs_test.go` 先写 `DesktopPrefs` 测试（缺省归一化 trayResident=true/autoStart=false/autoUpdateCheck=true；旧格式 prefs.json 无 desktop 字段可读；round-trip 原子写），FAIL 后在 `internal/prefs/prefs.go` 实现 `Desktop *DesktopPrefs` 字段与 `NormalizeDesktop()`（data-model §1.1）
- [x] T007 TDD：在 `desktop/bridge_test.go` 先写桥接路由测试（非 `/api/v1/` 前缀 → `404 {"error":"not found"}` 且**不得返回 HTML**；`/api/v1/*` 其余 → 透传 fake handler；契约见 contracts/desktop-api.md §1），FAIL 后在 `desktop/bridge.go` 实现路由核心（构造函数注入下游 handler，纯 net/http 可测）
- [x] T008 [P] TDD：在 `desktop/info_test.go` 先写 `GET /api/v1/desktop/info` 契约测试（mode/appVersion/embeddedCliVersion 字段、cli 探测经注入 fake、响应结构见 data-model §2.1），FAIL 后在 `desktop/info.go` 实现（CLI 探测器定义为接口，便于 US2 替换真实实现）

**Checkpoint**: Foundation ready — user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - 打开应用即完成配置 (Priority: P1) 🎯 MVP

**Goal**: 双击打开原生窗口，界面与 `ccs gui` 同源，provider 配置全程可用；既有功能零回归

**Independent Test**: 打包产物双击启动 → 完成 provider 增改删 → 终端 `ccs use` 验证同一份数据生效；`make check && make test && make e2e` 全绿（quickstart §2 Story 1）

### Implementation for User Story 1

- [x] T009 [US1] 实现 `desktop/main.go` 应用装配：`application.New`（窗口 + `AssetServer{Assets: internal/web.AssetsFS(), Handler: desktop.NewBridge(...)}` + 生命周期），版本经 ldflags 注入 `internal/version.Version`（research R1）
- [x] T010 [US1] 单实例：v3 SingleInstance 配置，二次启动回调聚焦既有窗口（FR-011，research R6）
- [x] T011 [US1] `wails3 dev` 开发流打通（dev server 指向 vite；quickstart §1 命令实测可用，差异写入 desktop/README.md）
- [x] T012 [US1] 验证检查点：跑 quickstart §2 Story 1 手工脚本 + 全量回归（`make check && make test && make e2e`，FR-003/SC-003 零回归证明）

**Checkpoint**: MVP 成立——桌面形态独立交付价值

---

## Phase 4: User Story 2 - 一键安装内置 ccs 到 PATH (Priority: P2)

**Goal**: 设置区「安装到 PATH」按钮完成内置 CLI 安装，四态语义（干净/同版/升级/外来源）全覆盖

**Independent Test**: 干净 PATH 安装 → 新终端 `cc-select --version` 为内置版本；重复/升级/外来源/无权限四场景按 contracts §3.3 响应（quickstart §2 Story 2）

### Tests for User Story 2 ⚠️（先写并确认 FAIL）

- [ ] T013 [P] [US2] `desktop/cliinstall_test.go`：版本探测与安装四态测试（探测器/命令执行器接口 fake + `t.TempDir` 模拟 PATH 目录；覆盖 clean/reinstalled/upgraded/external-409/permission-fallback，契约 contracts §3.3）
- [ ] T014 [P] [US2] `desktop/bridge_test.go` 增补：`POST /api/v1/desktop/cli/install` 契约测试（202 语义、`confirmOverwrite` 流转、错误结构）

### Implementation for User Story 2

- [ ] T015 [US2] 实现 `desktop/cliinstall.go`：探测（`exec.LookPath`+`--version`）与安装（mac/linux `/usr/local/bin`→`~/.bin` 回落；win `%LOCALAPPDATA%\Programs\cc-select\` + HKCU Path 写入），使 T013 GREEN
- [ ] T016 [US2] bridge 挂接 install 端点 + `info` 端点接入真实探测器（T008 的 fake 换真实现），使 T014 GREEN
- [ ] T017 [US2] go:embed 内置 CLI 二进制 + `embeddedCliVersion` 注入（Makefile T003 链路贯通，缺失即构建失败，research R3）
- [ ] T018 [P] [US2] 前端 `internal/frontend/src/components/DesktopSettings.tsx`：Desktop 设置区骨架（`/desktop/info` 探测 404→隐藏，FR-015 条件渲染）+ 安装按钮四态文案/确认弹窗，配 `DesktopSettings.test.tsx`（vitest）
- [ ] T019 [US2] 验证检查点：quickstart §2 Story 2 四场景手工脚本

**Checkpoint**: Stories 1+2 独立可用

---

## Phase 5: User Story 3 - 应用内检查并一键更新 (Priority: P3)

**Goal**: 更新卡片 UX 不变，底层换 Wails Updater（下载→校验→原子替换→重启），启动自动检查可关

**Independent Test**: mock release（0.1.1 + SHA256SUMS + manifest）下，0.1.0 启动→提示→一键更新→重启为新版，数据无损；断网温和降级（quickstart §2 Story 3）

### Tests for User Story 3 ⚠️（先写并确认 FAIL）

- [ ] T020 [P] [US3] `desktop/updatebridge_test.go`：check 响应与 web 端点字段同构映射（fake updater service）、POST 202/409（进行中锁）/422（校验失败）、失败降级不 5xx（契约 contracts §2）
- [ ] T021 [P] [US3] `desktop/settings_test.go`：`GET/PUT /api/v1/desktop/settings` 契约（204 幂等、400 非法字段、503 自启不可用回滚不落盘，契约 contracts §3.2；服务同步用回调注册 fake）

### Implementation for User Story 3

- [ ] T022 [US3] 实现 `desktop/updatebridge.go`：拦截 `/api/v1/update*` → Wails Updater（GitHub provider、CurrentVersion、`ChecksumAsset:"SHA256SUMS"`、headless），使 T020 GREEN；**红线断言**：不调用 `internal/updater` 替换逻辑（research R2）
- [ ] T023 [US3] 实现设置端点 `desktop/settings.go`（prefs 落盘 + 服务回调注册机制：本故事挂 autoUpdateCheck 回调，US4/US5 后续注册托盘/自启），使 T021 GREEN
- [ ] T024 [US3] 启动时按 `autoUpdateCheck` 静默检查 + 发现新版经通知服务提醒（FR-006）
- [ ] T025 [P] [US3] 前端 DesktopSettings 增「自动检查更新」开关（PUT settings；沿用既有 CheckUpdateButton/更新卡片，零改动验证），配 vitest
- [ ] T026 [US3] 验证检查点：quickstart §2 Story 3 全脚本（含断网与中断恢复）

**Checkpoint**: Stories 1–3 独立可用

---

## Phase 6: User Story 4 - 托盘常驻与快捷切换面板 (Priority: P4)

**Goal**: 关窗驻留托盘（可改关窗即退出）；托盘列 providers，点击复制 `ccs use` 命令 + 通知；R1 红线零违反

**Independent Test**: 关窗→托盘存在；点 provider→剪贴板为命令；双终端用不同 provider 时托盘操作后各自 `ccs current` 不变（quickstart §2 Story 4）

### Tests for User Story 4 ⚠️（先写并确认 FAIL）

- [ ] T027 [P] [US4] `desktop/tray_test.go`：`GET /api/v1/desktop/tray/commands` 数据契约测试（来自 providers 配置、含 copyCommand、**无任何执行切换动作**——FR-009 契约层锁定，contracts §3.4）

### Implementation for User Story 4

- [ ] T028 [US4] 实现 `desktop/tray.go`：菜单装配（providers→复制命令+系统通知、打开切换面板、状态、退出）+ `trayResident` 行为（关窗驻留 vs 退出，经 settings 回调注册）+ 无托盘环境优雅降级，使 T027 GREEN
- [ ] T029 [P] [US4] 前端 DesktopSettings 增「关闭窗口时驻留托盘」开关 + providers 列表随配置变更刷新（读时拉取，Edge Case「运行中变更可见」），配 vitest
- [ ] T030 [US4] 验证检查点：quickstart §2 Story 4 手工脚本（**含 R1 红线双终端验证**）

**Checkpoint**: Stories 1–4 独立可用

---

## Phase 7: User Story 5 - 开机自启 (Priority: P5)

**Goal**: 「登录自启」开关（默认关）三平台一致；自启以托盘形态启动不抢焦点

**Independent Test**: 开关切换后注销重登行为正确；三平台（Linux 验 XDG 文件生成/移除）（quickstart §2 Story 5）

### Implementation for User Story 5

- [ ] T031 [US5] TDD + 实现：`desktop/autostart_test.go`（失败回滚：fake autostart 服务返回错误→prefs 不落盘已改字段）→ 接入 v3 `app.Autostart` 服务与 settings 回调（research R6）
- [ ] T032 [P] [US5] 前端 DesktopSettings 增「登录自启」开关，配 vitest
- [ ] T033 [US5] 验证检查点：quickstart §2 Story 5 手工脚本

**Checkpoint**: 全部 stories 独立可用

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: 发布流水线、文档、回归与评审（跨全部 stories）

- [ ] T034 [P] 发布流水线：`.github/workflows/` 扩展三平台 matrix（macos-arm64/windows-x64/ubuntu），产物命名/SHA256SUMS/manifest 生成与 CI 断言（体积 ≤40MB、命名正则、sha256 一致，contracts/release-assets.md §2/§4/§6）
- [ ] T035 [P] 文档：README（桌面端安装/使用 + macOS「右键打开」+ Windows SmartScreen 指引）+ `docs/language/README.*.md` 各语言同步（宪法 i18n 约定）+ `desktop/README.md` 开发指南
- [ ] T036 全量回归 + 三平台冒烟：`make check && make test && make e2e` + quickstart §3 矩阵全项
- [ ] T037 循环 code review 并修复**中等严重及以上**问题直至清零（宪法 Development Workflow；含 `/speckit-analyze` 交叉校验）
- [ ] T038 宪法 VII 回流：验收结论回填 docs/（acceptance-tests.md 桌面用例、requirements.md 变更记录、tech-stack.md 状态、roadmap 如涉及）

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: 无依赖，立即开始；T001 先行（T003/T004 产物依赖其存在才有意义）
- **Foundational (Phase 2)**: 依赖 Phase 1 —— **阻塞全部 stories**（T005–T008 为纯 Go 基座）
- **User Stories (Phase 3–7)**: 均依赖 Phase 2 完成
  - US1 必须最先做（后续 stories 的设置区/更新流都跑在 US1 的窗口壳里）
  - US2–US5 相互独立，可并行（若单人则按 P2→P5 顺序）
- **Polish (Phase 8)**: 依赖全部预期 stories 完成

### User Story Dependencies

- **US1 (P1)**: Phase 2 后即可开始，无跨故事依赖 —— **MVP 边界**
- **US2 (P2)**: 依赖 US1 的窗口壳与 T018 设置区骨架；自身功能独立可测
- **US3 (P3)**: 依赖 T023 设置端点（自身交付）；更新卡片 UI 沿用既有组件
- **US4 (P4)**: 复用 US3 的设置端点机制（回调注册）——若并行开发，US4 可自带临时开关，集成时切换
- **US5 (P5)**: 同 US4；且建议在 US4（托盘）后做，自启默认托盘形态依赖托盘就绪

### Within Each User Story

- Tests MUST be written and FAIL before implementation（宪法 VIII）
- 探测器/服务均以接口注入（T008/T015/T022/T031 的 fake 即为此设计）
- **一个功能点一个 commit**（项目开发基本要求）；测试与实现同 commit

### Parallel Opportunities

- Phase 1：T002/T003/T004 可并行
- Phase 2：T006 与 T005/T007/T008 可并行（不同包/文件）
- US2 内：T013/T014 测试并行；T018 前端与 T015–T017 后端并行
- US3 内：T020/T021 并行；T025 前端与 T022–T024 后端并行
- 单人执行推荐顺序：T001→T002/T003/T004→T005→T006/T007/T008→US1→US2→US3→US4→US5→Polish

---

## Parallel Example: User Story 2

```bash
# 并行写出两份测试（不同文件）：
Task T013: "cliinstall 四态测试 in desktop/cliinstall_test.go"
Task T014: "install 端点契约测试 in desktop/bridge_test.go"

# 后端与前端并行：
Task T015–T017: "desktop/cliinstall.go + bridge 挂接 + embed 链路"
Task T018: "DesktopSettings.tsx + vitest"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 1 Setup + Phase 2 Foundational
2. Phase 3 US1 → **STOP and VALIDATE**（quickstart §2 Story 1 + 全量回归）
3. 此时可演示：桌面形态开箱即配置

### Incremental Delivery

1. Foundation → US1（MVP）→ 验证
2. +US2 安装 CLI → 验证 → +US3 自更新 → 验证
3. +US4 托盘 → 验证 → +US5 自启 → 验证
4. Phase 8 发布流水线 + 文档 + 评审 + 回流

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to spec.md user story（US1=打开即配置 / US2=安装 CLI / US3=更新 / US4=托盘 / US5=自启）
- 每个 US 的「验证检查点」任务以 quickstart.md 对应脚本为准，通过才进入下一 story
- Wails API 细节以锁定的 v3.0.0-beta.34 官方文档为准（research.md 参考链接）；Beta API 如有出入，以最小偏差适配并在 commit message 注明
