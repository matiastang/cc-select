---
description: "Task list for feature implementation"
---

# Tasks: UI/UX 改版与快速上手（v0.1.1）

**Input**: Design documents from `/specs/004-ui-ux-redesign/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: **强制包含**（宪法原则 VIII：TDD 先行——每个测试任务先写并确认 FAIL，再实现至 GREEN）。项目开发基本要求另规定：一个功能点一个 commit。

**Organization**: Tasks are grouped by milestone (M1 设计系统/UX 改版 → M2 远端预设+连通性 → M3 限额查询 → 收尾)。

**分支约定**：实现自 `dev/tdy` 切出 `004-ui-ux-redesign` 特性分支（宪法 Development Workflow）。

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: spec.md 用户故事（US1–US7）
- Include exact file paths in descriptions

## Path Conventions

- 前端：`internal/frontend/`（src、e2e、package.json、vite/postcss/tailwind 配置）
- Go：`internal/{presets,prefs,web,probe,usage}/`、`Makefile`
- 契约真值：`specs/004-ui-ux-redesign/contracts/`

---

## Phase 1: M1 — 设计系统与 UX 改版

**Purpose**: Tailwind/token 基建 → 基件 → 导航壳 → 模块迁移 → 反馈系统 → 画廊/向导 → 预设单源数据

- [x] T001 [US2] 前端基建：`internal/frontend` 安装 tailwindcss@^3.4 + postcss + autoprefixer + tailwindcss-animate + clsx + tailwind-merge + lucide-react；`tailwind.config.ts`/`postcss.config.js`；`src/lib/cn.ts`；`src/style.css` 重构为 token 层（CSS 变量族沿用 + `@layer base/components`，深浅色三态结构不变，`theme.test.ts` 保持绿）。既有 vitest 全绿为基线。
- [x] T002 [US2] 基件重写（表单/展示类）：`src/components/ui/{Button,Input,Textarea,Select,Label,Card,Badge,Spinner,EmptyState,FormField,Collapsible,SegmentedControl}.tsx` 以 Tailwind 重写，导出名与既有 props 兼容；先改/扩各 `*.test.tsx` 至红再实现。`Icon.tsx` 保留转发层（内部映射 lucide）。
- [x] T003 [US2] Radix 基件：`ui/{Dialog,Tabs,Switch,Tooltip,Toast,Progress,Stepper}.tsx` 新建（Stepper 自研；Toast 含 `useToast` context）；配 vitest。依赖按 `contracts/frontend §1` 逐个引入。
- [x] T004 [US1] 导航：`src/navigation/{router.ts,registry.ts}` hash router + 模块注册表（`NavModule` 契约、`nav-{route}` testid、desktopOnly 条件注册钩子）；`router.test.ts`/`registry.test.ts` 先行。
- [x] T005 [US1] 布局壳：`App.tsx` 重构为壳 + 路由出口；侧边栏（≥880px）/顶部 tab（<880px）双形态同源渲染；Header 保留主题/语言切换（`header-controls` testid 不变）；既有组件**原样挂入**各模块（中转态零回归）；`ShellIntegrationBanner` 留 providers 首页；notice 移 about。既有 4 个 e2e spec 修复（仅允许补 `nav-*` 导航步骤）。
- [x] T006 [US1] 模块定稿迁移：`src/features/{mode-routes,security,desktop,general,about}/` 目录化（组件迁移 + i18n key 归位 `general.json` 新命名空间）；每模块 vitest 随迁。
- [x] T007 [US2] 反馈系统：App 级 error → `useToast`；ProviderList 空状态（`providers-empty-state`）引导至画廊；各模块保存/删除反馈接 toast；vitest 断言 role="status"。
- [x] T008 [US3] 预设画廊：`src/features/providers/PresetGallery.tsx`（分类分组卡片 + `preset-gallery-*` testid + custom 入口）；vitest 先行。
- [x] T009 [US3] 添加向导：`src/features/providers/ProviderWizard.tsx`（Stepper：Step1 名称+API key+获取 key 外链+「测试连接」占位；Step2 高级折叠=隔离模式/模型覆盖预填/EnvFieldEditor/JSON 视图复用）；`add-provider-button` 保留触发画廊；编辑路径复用同表单回显；`JsonForm` 逻辑收编为 wizard 的模型（保留既有校验语义）；vitest 覆盖 ≤3 步保存与必填拦截。
- [x] T010 [US4] 预设单源化（Go）：先写 `internal/presets/{validate_test.go,source_meta 骨架测试}`；迁移 13 条至 `internal/presets/presets.json` + 新增 `minimax`、`qwen-coding` + GLM 双条目模型保鲜 `glm-5.3` 系（数据事实=contracts/presets-and-usage §1.3）；`validate.go` 整包校验；`presets.go` 重构为 embed 加载，`All/ByID/BuildEnv` 签名不变；Go 单测全绿（含 CLI add 路径回归）。

## Phase 2: M2 — 远端预设热更新 + 连通性测试

- [x] T011 [US4] `make check` 增 `presets-lint`（Go 小命令或 test 钩子调用同一 validate 校验仓库内 presets.json；坏文件无法过 CI）。
- [x] T012 [US5] 偏好开关：`internal/prefs` 增 `RemotePresetsDisabled`（TDD：prefs_test 先行）；`internal/web` 增 `GET|PUT /api/v1/remote-presets`（api_test 先行）。
- [x] T013 [US5] 源解析状态机：`internal/presets/source.go`（builtin→cache→remote、24h TTL、5s 拉取超时、锁定 URL+拒跨 host 重定向、原子写缓存 0600、`Refresh()`、`Meta()`）；httptest 覆盖 更新/不可达/损坏/禁用 四情形（先红后绿）；`GET /presets` 响应增 `meta`、`POST /presets/refresh` 端点。
- [x] T014 [US5] 前端预设源 UI：general 模块 `RemotePresetsCard`（开关 `remote-presets-toggle` + 手动刷新 + source/fetchedAt 展示，`presets-source-badge`）；画廊头部 source 徽标；vitest。
- [x] T015 [US6] `internal/probe/client.go`：`SendMessage`（超时 10s、不重试）+ 错误分类 auth/network/model/server/unknown（消息剥 token 截 200 字符）；httptest 先行。
- [x] T016 [US6] `POST /api/v1/providers/probe` 端点（契约 api.md §2.3：缺认证/缺模型 400 语义、官方无 base 时默认模型）；api_test 先行。
- [x] T017 [US6] 前端「测试连接」：向导 Step1 与 provider 详情 `probe-test-button`（四态：空闲/进行/成功含延迟/失败分类文案）；vitest。

## Phase 3: M3 — 限额查询（R11）

- [x] T018 [US7] `internal/usage`：`Report/Window/Adapter` + 注册表（首个 Match 生效 + 兜底恒匹配）；`anthropic_headers.go` 通用头探测（复用 probe client；unified 多窗口与独立头两形态；零窗口→unsupported）；httptest 先行。
- [x] T019 [US7] 专用适配器：`glm_quota.go`（`/api/monitor/usage/quota/limit`、Authorization 原样、TOKENS_LIMIT→5h/TIME_LIMIT→month-mcp percent-only）、`kimi_balance.go`（`/v1/users/me/balance` Bearer→balance 窗口；api.kimi.com 不注册）；httptest 按研究事实先行。
- [x] T020 [US7] `POST /api/v1/usage/{id}` 端点（404/500/unsupported 语义，契约 api.md §2.4）；api_test 先行。
- [x] T021 [US7] 前端 `UsagePanel`：provider 卡片展开区（`usage-refresh-button`/窗口 Progress+percent+resetAt 倒计时/`usage-unsupported` 占位/fetchedAt）；vitest。

## Phase 4: 收尾（回归/评审/回流）

- [x] T022 e2e 全量：新增 `navigation.spec.ts`、`add-wizard.spec.ts`、`presets-refresh.spec.ts`、`probe.spec.ts`、`usage.spec.ts`；既有 4 spec 全绿（testid 契约校验——contracts/frontend §5）。
- [x] T023 i18n 核查：en/zh 新增 key 全量对齐（既有 locales 完整性测试模式扩展到 `general.json`）。
- [x] T024 完整回归：`make check` + `make test` + `make integration` + `make e2e` + `make desktop-build`（darwin 本机）；记录结果于本文档勾选说明。
- [x] T025 循环 code review：静态检查 + 逐模块评审（宪法 Development Workflow），修复全部中等及以上问题并复评至零遗留。
- [x] T026 文档回流：`docs/acceptance-tests.md` 增 AC19；`docs/tech-stack.md` 记录前端依赖决策；`docs/roadmap.md` 状态更新；`CLAUDE.md` 前端结构段更新；`docs/requirements.md` 变更记录；宪法 Sync Report（依赖事实 PATCH）；004 tasks 勾选归档。

---

## 状态

- T024 回归记录：`make check` 全绿；`make test` 17 包 ok；`make integration` 18 包 ok（含 desktop，CGO 修复）；Playwright 19/19；vitest 118/118；`make desktop-build`（darwin）成功。
- T025 评审记录：四轮循环闭合——第一轮 3 高 + 6 中 + 9 低（9c082f8 修复）；第二轮复评新增 2 中 + 6 低（44d909a 修复）；第三轮残余 3 中 + 4 低（8bf5e4c 修复）；第四轮终验确认 critical 0 / high 0 / medium 0（含 -race 通过）。

- 里程碑串行：M1 → M2 → M3 → 收尾；Phase 内 [P] 任务可并行。
- 每 task = 一 commit（Conventional Commits，scope 建议 `frontend`/`presets`/`probe`/`usage`/`web`/`prefs`/`e2e`/`docs`）。
