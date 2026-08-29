# Implementation Plan: model-picker-sync（Mode P 会话内真实模型选择与显示）

**Branch**: `002-model-picker-sync` | **Date**: 2026-08-29 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/002-model-picker-sync/spec.md`

## Summary

在 profile `settings.json` 生成/重建时注入 Claude Code `modelPicker`（模型清单由 provider env 四模型变量派生，`replaceBuiltInOptions: true`，原始 id 无 label），Mode P 额外注入 `model` 字段使当前模型标记/横幅显示真实主模型；`route switch` 热切时按继承的 `$CLAUDE_CONFIG_DIR` 定位**发射 profile** 的 settings.json，读-改-写原子刷新 `modelPicker`+`model`（CC 的 modelPicker 已实验证实会话内热重载）；代理 `ModelRewrite` 从「无条件主模型」升级为「清单透传 → 槽位子串分类映射 → 主模型回落 → 无主模型透传」的映射化规则。单一 ModelPlan 派生点保证显示与路由永远同源。

## Technical Context

**Language/Version**: Go 1.24（沿用，无新依赖——纯标准库 JSON 操作）

**Primary Dependencies**: 无新增。既有：cobra / go-keyring / net/http / embed（原则 IV 选型不漂移）

**Storage**: 无新存储文件。产物写入既有 `~/.cc-select/profiles/<id>/settings.json`（契约见 [contracts/profile-settings.md](./contracts/profile-settings.md)）

**Testing**: Go 单测（`go test ./internal/...`）+ 集成测试（`internal/router` httptest 双端，沿用 001 模式）；TDD red-green-refactor（宪法 VIII）

**Target Platform**: macOS / Linux / Windows 三平台同时成立（宪法 III）；`route switch` 的 `$CLAUDE_CONFIG_DIR` env 继承机制 = 001 研究 D6 已确立的 `CC_SELECT_TID` 同信道

**Project Type**: CLI（Go 单二进制 + 内嵌 Web GUI；本特性不触前端）

**Performance Goals**: daemon 逐请求改写开销不增（分类为 O(1) 子串匹配，JSON 解析沿用现状）；`use`/`route switch` 增加一次 settings.json 读-改-写（<10ms 量级）

**Constraints**: profile settings.json 原子写（沿用 `EnsureRaw` 模式）；Mode P profile 零密钥（宪法 II/INV-1）；32 MiB body 上限防护保留；不触碰 SSE/非 Messages 端点透传语义（001 既有）

**Scale/Scope**: 每 provider ≤4 模型行（env 派生，spec Assumption Q11）；单终端 settings.json 刷新频率 = 热切次数级

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| 原则 | 判定 | 说明 |
|---|---|---|
| I. Eval/Wrapper 拆分 | ✅ | `route switch` 保持普通 CLI（打印结果 + 写文件），刷新是**文件写入**而非 env 修改，不触碰父 shell；`use` 的 eval 语句形态不变 |
| II. CLAUDE_CONFIG_DIR 隔离 + Mode P 条款 | ✅ | 注入/刷新只写 profile settings.json 文件；Mode P profile 仍只含代理 BASE_URL + 模型名（零密钥，INV-1）；路由真值仍是 routes.json + daemon，显示刷新失败不阻断切换（与「显示是增强」语义一致） |
| III. 三平台 | ✅ | 刷新路径纯文件操作 + env 读取，无 shell/链接/编码差异；PowerShell 下 `CLAUDE_CONFIG_DIR` 由 emitter 注入、子进程继承语义一致（D6 同信道）；验收用例需含 Windows PowerShell 集成步骤（既有 CI 覆盖） |
| IV. 技术选型不漂移 | ✅ | 无新依赖、无新存储选型、无 GUI 形态变化 |
| V. 质量门禁 | ✅ | 行为变更将同步 `docs/acceptance-tests.md`（新增 AC17）；测试分层沿用 |
| VI. 安全基线 | ✅ | 模型名非凭证；keychain 占位在派生时显式排除 |
| VII. 文档链真值 | ✅ | 需求源自 requirements.md R10（Q11–Q13 默认倾向落入 spec Assumptions）；实现验收后回流 docs（含 engineering-decisions §8 env 冻结表述修正——见 research D9） |
| VIII. TDD | ✅ | tasks 阶段按 red-green-refactor 排序；modelrewrite 改造属行为变更，先失败测试锁定 v1→v2 语义差异 |

**结论**：无违例，Complexity Tracking 不适用。

## Project Structure

### Documentation (this feature)

```text
specs/002-model-picker-sync/
├── plan.md              # 本文件
├── research.md          # Phase 0 决策记录
├── data-model.md        # Phase 1：ModelPlan / 字段所有权 / 改写状态机
├── quickstart.md        # Phase 1：五场景端到端验证指南
├── contracts/
│   ├── profile-settings.md    # settings.json 注入契约（形状/不变量/写入时机）
│   └── proxy-model-routing.md # 代理改写算法契约（可观察行为）
└── tasks.md             # Phase 2（/speckit-tasks 产出，非本命令）
```

### Source Code (repository root)

```text
internal/
├── config/
│   └── modelplan.go          # 新增：ModelPlan 派生（纯函数，Provider.Env → ModelPlan）
│   └── modelplan_test.go     # 新增：派生规则单测（去重/占位跳过/全空）
├── profile/
│   ├── merge.go              # 改动：mergeSettings 注入 modelPicker（+Mode P 的 model）
│   ├── build.go              # 改动：Sync/SyncProxy 传入 Mode P 真值源（providers.json Env）
│   └── modelpicker_test.go   # 新增：注入形状/不变量 INV-1~5 单测
├── cli/
│   ├── route.go              # 改动：runRouteSwitch 成功后刷新 $CLAUDE_CONFIG_DIR/settings.json
│   └── route_cli_test.go     # 改动：刷新路径用例（含 env 未设/文件损坏/字段保留）
└── router/
    ├── modelrewrite.go       # 改动：R1~R6 映射化改写算法
    └── modelrewrite_test.go  # 改动：契约示例表全量用例 + v1 回归用例
```

**Structure Decision**：单项目 CLI 结构，无新目录。ModelPlan 派生放 `internal/config`（Provider 的方法域，router/profile/cli 三处消费者共用）；注入逻辑放 `internal/profile`（settings.json 的唯一生产者域）；刷新逻辑放 `internal/cli/route.go`（切换命令的既有边界内，001 已确立「route = 普通 CLI」语义）。
