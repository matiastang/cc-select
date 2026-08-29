# Implementation Plan: 会话内切换 provider（In-Session Provider Switch）

**Branch**: `001-in-session-provider-switch` | **Date**: 2026-08-28 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-in-session-provider-switch/spec.md`

## Summary

R9 两个交付层：

- **P0（US2，基线）**：把「退出 → `ccs use B` → `claude --continue`」跨 provider 续会话确立为受验收的标准工作流——补 `docs/acceptance-tests.md` 用例 + README/isolation-modes 文档说明。零新组件，依赖 Mode B 既有的 `projects/` 共享链接。
- **P1（US1/3/4，完整目标）**：新增 **Mode P（proxy，代理路由模式）**，opt-in 第三隔离模式。核心机制：「身份与路由分离」——`ccs use` 后 claude 的 env 恒定（`ANTHROPIC_BASE_URL`=本地路由守护进程、`ANTHROPIC_AUTH_TOKEN`=per-terminal 伪 token `CC_SELECT_TID`），真正去哪家由路由表 `~/.cc-select/routes.json`（tid → provider）决定。会话内切换 = `cc-select route switch <id>` 改路由表（改文件，不改任何进程 env，绕开「子进程不能改父进程 env」与「claude env 冻结」两条硬约束）。真 token 迁移至 keychain（US4）。`current` 在 Mode P 下读路由表保证状态一致（US3）。

## Technical Context

**Language/Version**: Go 1.24（CLI + daemon）+ 内嵌 React/TS 前端（GUI 增量）

**Primary Dependencies**: cobra（CLI，既有）；`net/http` + `net/http/httputil.ReverseProxy`（daemon，标准库）；zalando/go-keyring（keychain，既有）；无新增第三方依赖

**Storage**: 既有 JSON（`providers.json` / `prefs.json` / `profiles/<id>/settings.json`，原子写）+ 新增 `~/.cc-select/routes.json`（路由表）与 `~/.cc-select/router.json`（daemon 状态）；密钥真值迁 OS keychain（service `cc-select:<id>:<var>`，既有约定）

**Testing**: `go test ./internal/...`（单测）+ `-tags integration`（daemon ↔ fake upstream 全链路）+ Playwright e2e（GUI mode/路由面板）；TDD 强制（宪法 VIII）

**Target Platform**: macOS / Linux / Windows（PowerShell only）；daemon 仅绑定 127.0.0.1

**Project Type**: cli + local web GUI + **local loopback router daemon（新增形态，opt-in）**

**Performance Goals**: `route switch` < 1s（本地文件写）；代理转发开销 < 10ms（loopback）；SSE 流式 flush 即时（`FlushInterval` 立即模式）

**Constraints**: 无 CGO；不引入常驻服务安装（launchd/systemd/计划任务不做，按需拉起）；i18n en/zh；单二进制形态不变（daemon 是同一 binary 的子命令）

**Scale/Scope**: 单用户单机；provider 数十；并发终端 ≤ 个位数；路由表条目 = 历史终端数（prune 收敛）

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| 原则 | 判定 | 说明 |
|---|---|---|
| I. Eval/Wrapper 拆分 | ✅ PASS | `route switch` 是**普通子命令**（打印结果、写文件），不输出 eval 语句、不试图改父 shell env；`use` 仍走 eval。会话内触发正是利用「Bash 子进程可写文件、改不了 env」的边界 |
| II. CLAUDE_CONFIG_DIR 路由 | ⚠️ **需修订（MINOR）** | Mode P 的 provider 路由经 daemon 路由表而非 CLAUDE_CONFIG_DIR 切换；`current` 在 Mode P 读路由表（per-terminal 真值，非全局模板——原则意图保持）。需按治理流程将宪法 1.1.0 → 1.2.0，增加 Mode P 豁免条款；**docs/（R9 已确认方向）为准，宪法随后修订**，见 Complexity Tracking |
| III. 三 OS | ✅ PASS | detached spawn（Unix setsid / Windows DETACHED_PROCESS）、junction 复用、PowerShell 发射器、go-keyring 三平台 |
| IV. 选型不漂移 | ✅ PASS | 标准库反代 + 既有依赖；不新增框架；daemon= 同一 Go binary |
| V. 质量门禁与验收同步 | ✅ PASS | 单测+集成+e2e 分层齐备；acceptance-tests.md 增 P0/P1 用例 |
| VI. 安全基线不降级 | ✅ **改善** | Mode P 把 token 迁 keychain（既有升级路径）；daemon 仅 loopback + 伪 token 鉴权；日志无密钥 |
| VII. 文档链真值 | ✅ PASS | 需求→spec→plan 链完整；实现后结论回流 docs/（isolation-modes.md 增 Mode P、engineering-decisions 增 §） |
| VIII. TDD | ✅ PASS | tasks.md 每个 Implementation Task 必须先写失败测试 |

**Gate 结论**：原则 II 的字面偏离已由 docs/requirements.md R9（用户确认）授权，属「docs 赢、宪法随即修订」的既定治理路径，非未授权违规 → **放行**。

## Project Structure

### Documentation (this feature)

```text
specs/001-in-session-provider-switch/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   ├── cli.md           # 新增/变更 CLI 命令契约（含发射 env 契约）
│   ├── router-http.md   # 路由 daemon 的 HTTP 行为契约
│   └── web-api.md       # Web GUI REST 增量契约
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/
├── prefs/               # Mode 增加 ModeProxy（"proxy"）枚举 + ResolveMode 兼容
├── routes/              # NEW 路由表存取：tid → provider（原子写 0600、prune）
│   ├── routes.go
│   └── routes_test.go
├── router/              # NEW Mode P 核心：本地路由守护进程
│   ├── server.go        # loopback 监听、伪 token 鉴权、/healthz
│   ├── state.go         # router.json 状态 + ensureDaemon（健康探测/拉起/复用）
│   ├── forward.go       # ReverseProxy：真 token 注入、头改写、SSE 透传
│   ├── modelrewrite.go  # 请求体 model 字段改写（目标 provider 的 ANTHROPIC_MODEL）
│   └── *_test.go        # 含 -tags integration 的 fake upstream 全链路测试
├── switcher/            # Plan 扩展：proxy 模式发射（BASE_URL 经 profile、TID 守卫导出）
├── shell/               # Change 增加守卫式 Set（OpSetIfUnset），zsh/bash + PowerShell 渲染
├── profile/             # Sync 扩展：Mode P profile（env 仅 BASE_URL + Mode B 共享链接）
├── cli/
│   ├── route.go         # NEW route switch/list/status/prune（会话内触发入口）
│   ├── router.go        # NEW router ensure/serve/stop（daemon 生命周期）
│   ├── use.go           # proxy 模式集成：ensure + 写路由 + 发射
│   └── current.go       # CC_SELECT_TID 存在时以路由表为真值
├── web/                 # /api/v1/mode 接受 "proxy"；NEW /api/v1/routes（GET/prune）
└── i18n/                # 新增文案 en/zh
docs/
├── acceptance-tests.md  # P0 跨 provider 续会话用例 + P1 路由隔离用例
├── isolation-modes.md   # 增 Mode P 章节
└── engineering-decisions.md  # 增「会话内切换与身份/路由分离」§
.specify/memory/constitution.md  # 1.1.0 → 1.2.0（原则 II Mode P 豁免条款）
```

**Structure Decision**: 沿用单 module 单 binary 布局；新增 `internal/routes`（纯存储）与 `internal/router`（daemon 进程逻辑）两个包，职责与既有 `config`/`profile` 对等；CLI/web/i18n 做增量扩展，不新建顶层目录。

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| 宪法 II 字面偏离（Mode P 路由不经 CLAUDE_CONFIG_DIR） | R9-P1 要求会话内切换；claude env 冻结 + 子进程不可改父 env 两条物理约束下，唯一可行解是把路由决策移出 claude 进程（身份/路由分离） | 「热重载 settings.json env」官方不支持（issue #62656）；「退出重进」只能达 P0，达不到「不退出会话」的验收要求 |
| 引入常驻 daemon（产品形态变化） | BASE_URL 必须恒定且可转发，需要一个常驻监听进程承载路由决策 | 按需拉起 + 状态文件复用已把形态成本压到最低（免 launchd/systemd、崩溃自愈、单 binary 不变）；无 daemon 的替代方案不存在 |
| 请求体全量缓冲（model 改写需解析 JSON body） | 切 provider 后 claude 仍发送旧 model id，必须在代理侧改写 | 透传不改写会命中错误模型/404；仅 header 级方案无法改 body；本地 loopback 缓冲成本可忽略（<10ms 量级、内存单请求峰值） |
