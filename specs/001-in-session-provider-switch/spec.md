# Feature Specification: 会话内切换 provider（In-Session Provider Switch）

**Feature Branch**: `001-in-session-provider-switch`

**Created**: 2026-08-28

**Status**: Draft

**Input**: User description: "docs/requirements.md 中的 v0.0.6 / R9：Claude Code 会话内切换 provider（热切换，不丢上下文，不破坏 shell 级隔离，P0/P1 分级）"

> 需求真值：[docs/requirements.md §新增需求记录 → v0.0.6 → R9](../../../docs/requirements.md)。本 spec 只描述 WHAT/WHY；实现方向（本地路由代理等）是 R9 已记录的方向性决策，细节由后续 plan 阶段处理。

## Clarifications

### Session 2026-08-28

- Q: 会话内切换成功后，该终端之后新启动的会话应该用哪家 provider？ → A: **终端级生效**——切换即更新该终端的激活 provider，后续新会话沿用新 provider（与「一个终端任一时刻恰好一个生效 provider」的实体定义一致，避免退出后掉回已限额的旧 provider）。
- Q: 官方 provider 是否参与会话内热切？ → A: **v1 不参与**——新模式仅支持第三方 provider 之间的热切；使用官方时沿用现有模式或降级路径（代理 OAuth 凭据的复杂度与限额救急场景的价值不成比例）。
- Q: P0 三步工作流是否封装为一条组合命令？ → A: **不封装**——仅文档化 + 验收覆盖（`claude --continue` 本身已是一条命令，遵循 YAGNI）。

> 以上为跳过 `/speckit-clarify` 后在 `/speckit-plan` 阶段按 research.md D1–D3 记录的默认决策，可随时否决回改。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 限额救急：会话内热切 provider (Priority: P1)

我在终端 A 的 Claude Code 会话里使用 GLM 干活，GLM 的 5 小时用量窗口耗尽，模型请求开始报错。我不想丢掉进行到一半的任务，也不想退出会话重开。我在会话内执行一条切换命令（自己敲，或让 Claude 代跑），选择 MiniMax；之后的对话由 MiniMax 继续服务，而此前与 Claude 的全部对话内容依然可用，任务无缝继续——**全程不退出 Claude Code**。

（对应 R9 分级：P1 完整目标。）

**Why this priority**: 这是 R9 的核心价值主张——把「限额 = 任务中断 + 上下文风险」变成「限额 = 换一家接着干」。其余故事都是它的保障、兜底或衍生。

**Independent Test**: 配置两个可用 provider，在运行中的会话内发起切换并继续对话，验证（a）新 provider 服务后续请求、（b）切换前上下文可用、（c）会话未重启。单独实现本故事即可向用户演示核心价值。

**Acceptance Scenarios**:

1. **Given** 终端 A 的运行中会话正使用 provider X 且 X 已不可用（限额/故障），**When** 用户在会话内执行切换到 provider Y 的命令，**Then** 命令反馈成功，之后的模型请求由 Y 服务，且会话未被退出或重启。
2. **Given** 已成功切换到 Y，**When** 用户继续对话并引用切换前的内容，**Then** Y 能基于完整历史正确延续任务（上下文零丢失）。
3. **Given** 切换时终端 B 的会话正使用 provider Z，**When** 终端 A 完成切换到 Y，**Then** 终端 B 的后续请求仍由 Z 服务，不受影响（不破坏 R1）。
4. **Given** 目标 provider Y 未配置或配置无效，**When** 用户执行切换，**Then** 得到明确的失败原因，当前会话继续使用 X，不存在「切换了一半」的状态。
5. **Given** 切换瞬间有一笔流式响应尚未完成，**When** 切换完成，**Then** 该笔响应按原 provider X 完成，其后的新请求走 Y。

---

### User Story 2 - 降级路径：退出 → 切换 → 继续最近会话 (Priority: P2)

我暂时不启用（或用不上）会话内切换。当前 provider 到限额时，我退出 Claude Code、切换 provider、再以「继续最近会话」方式重新进入——对话上下文接得上，继续干活。这条三步路径是**被文档承诺、被验收用例保护的标准工作流**，而不是碰运气的隐藏行为。

（对应 R9 分级：P0 基线，先行交付，不依赖任何新能力。）

**Why this priority**: 零新增组件即可兑现 R9 的大部分价值；同时是 US1 的先行增量与永久兜底路径。

**Independent Test**: 在默认（共享）模式下按文档三步操作，验证最近会话可跨 provider 继续。可完全独立于 US1 实现与验收。

**Acceptance Scenarios**:

1. **Given** 默认共享模式下，终端 A 曾用 provider X 进行会话后退出，**When** 用户切换到 Y 并以「继续最近会话」方式重新进入，**Then** 历史完整呈现，新请求由 Y 服务。
2. **Given** 用户处于全隔离模式，**When** 查阅文档，**Then** 文档明确说明该模式跨 provider 续会话不适用及其原因。
3. **Given** 项目验收套件，**When** 运行，**Then** 存在覆盖本工作流的验收用例且通过。

---

### User Story 3 - 知道现在用的是谁 (Priority: P3)

会话内切换发生后，我能确知当前会话**实际**在用哪家 provider：切换命令的输出、状态查询、提示信息三者一致，不会出现「显示的还是旧 provider、实际请求已走新 provider」的误导。

**Why this priority**: 多终端 + 热切换之后，状态可见性（R6 的精神）从锦上添花变为正确使用的前提。

**Independent Test**: 会话内切换后查询当前状态，断言查询结果与最近一次切换结果一致。

**Acceptance Scenarios**:

1. **Given** 终端 A 的会话内刚切换到 Y，**When** 用户查询当前生效 provider，**Then** 结果为 Y，与切换反馈一致。
2. **Given** 终端 B 从未切换（仍为 Z），**When** 在终端 B 查询，**Then** 结果仍为 Z，未受终端 A 的切换影响。

---

### User Story 4 - 密钥不再明文躺盘 (Priority: P4)

启用新模式后，我检查机器上 cc-select 的配置文件，找不到任何 provider 真实密钥的明文。

（对应 R9 P1 派生要求，收敛 R7 已知风险。）

**Why this priority**: R7 的安全收敛要求，借新模式落地；未启用新模式时维持现状（既有风险已有文档标注，本特性不得使其扩大）。

**Independent Test**: 启用新模式并配置 provider 后扫描用户配置目录，断言无明文真实密钥。

**Acceptance Scenarios**:

1. **Given** 新模式已启用且 provider 已配置，**When** 检查用户配置文件内容，**Then** 不存在明文真实密钥。
2. **Given** 未启用新模式，**When** 检查，**Then** 行为与现状一致（不新增明文暴露面）。

---

### Edge Cases

- 官方 provider 不参与会话内热切（v1 范围决策，见 Clarifications）；需使用官方时切回现有模式或降级路径。
- 切换后想切回（原 provider 窗口恢复）：反向切换同等支持。
- 目标 provider 的模型名/能力与原 provider 不同：**不保证**行为完全一致，属文档说明范围，不算缺陷。
- 新模式所需辅助能力未就绪或中途异常：用户得到**可诊断**的错误与恢复指引，不静默失败；恢复后无需重开会话。
- 多终端同时各自切换：互不干扰（隔离语义的并发形态）。
- 切换命令在普通 shell（非 Claude Code 会话内）被执行：给出与场景匹配的反馈，不产生错误状态。
- 切换瞬间恰好有请求在途：见 US1 场景 5（在途请求按原 provider 完成）。

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 用户 MUST 能在**已运行**的 Claude Code 会话内，通过一条命令切换到任一已配置 provider，无需退出或重启该会话。
- **FR-002**: 切换 MUST 于**下一次**模型请求生效。
- **FR-003**: 切换后，当前会话的对话历史与任务进度 MUST 完整延续。
- **FR-004**: 会话内切换 MUST 只影响当前终端的会话；其他终端的后续模型请求 MUST 仍按各自 provider 服务。
- **FR-005**: 系统 MUST 将「退出 → 切换 provider → 继续最近会话」确立为受支持的标准工作流（文档 + 验收用例覆盖）；该工作流 MUST 在默认共享模式下成立，且 MUST 在全隔离模式下明确标注不适用。
- **FR-006**: 会话内切换能力 MUST 为可选启用；未启用时，现有切换行为与既有模式 MUST 完全不变。
- **FR-007**: 切换命令 MUST 给出明确的成功/失败反馈；失败时当前会话 MUST 能继续使用原 provider（切换具备原子性，无中间态）。
- **FR-008**: 系统 MUST 提供方式让用户确知某会话当前实际生效的 provider，且结果 MUST 与该会话最近一次成功切换一致。
- **FR-009**: 启用新模式后，provider 真实密钥 MUST NOT 以明文形式存在于用户配置文件中。
- **FR-010**: 本特性全部能力 MUST 在 macOS / Linux / Windows 三平台同等成立（Windows 仅 PowerShell，遵循既有约束）。
- **FR-011**: 新模式所需的辅助能力 MUST 对用户透明（自动就绪、无需手动管理），异常时 MUST 给出可诊断的错误信息与恢复指引。

### Key Entities *(include if feature involves data)*

- **终端会话（Terminal Session）**：一个终端里启动的 Claude Code 会话及其所属 shell 环境；具有可区分的身份；任一时刻恰好对应一个生效 provider。
- **生效 provider（Active Provider）**：某终端会话当前用于模型请求的服务商配置；随切换动作改变，且改变仅作用于对应终端会话。
- **会话历史（Conversation History）**：用户的工作产物，跨 provider 延续，不属于任何单一 provider。
- **Provider 配置（Provider）**：既有实体（R4）：端点地址、密钥、模型名等。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 「限额救急」演练：从发现当前 provider 不可用到在新 provider 上继续同一任务的下一句对话，用户操作 ≤ 1 条命令，且全程未退出会话。
- **SC-002**: 上下文延续性：切换后的对话能正确引用并继续切换前的任务内容（以验收场景脚本判定，历史零丢失）。
- **SC-003**: 隔离回归：双终端交叉切换各 ≥ 10 次的验收脚本中，终端间 provider 串扰次数为 0。
- **SC-004**: 降级路径：用户按文档操作，≤ 3 条命令完成「退出 → 切换 → 继续最近会话」且上下文延续。
- **SC-005**: 既有行为回归：未启用新模式时，全部既有验收用例通过（零行为变化）。
- **SC-006**: 切换命令自身完成时间 < 1 秒（不含模型响应时间）；对模型请求的流式输出无用户可感知的劣化。

## Assumptions

- 目标 provider 均提供 Claude Code 兼容接口（项目既有前提；R9 场景中的 GLM / MiniMax 均属此类）。
- 会话内切换的触发形态默认为**一条 CLI 命令**（R9 开放问题 Q8 的既定倾向）；MCP 工具、会话内自然语言触发为后续增强，不在本 spec 范围。
- 辅助能力生命周期默认「按需自动就绪」，不要求用户安装或手动启动常驻服务（Q9 的默认假设；具体产品形态在 plan 阶段确认）。
- 自动 failover（检测限额自动切换备用 provider）为**非目标**（Q10 已决策，仅记录为候选增强）。
- 「会话内切换」指模型请求路由的切换；不同 provider 的模型能力差异不在一致性保证范围内。
- 全隔离模式（Mode A）下跨 provider 续会话不适用——历史按 profile 隔离是既有设计；降级工作流仅在默认共享模式下承诺（R9 P0 已注明）。
- 依赖既有能力：provider 已按 R4 完成配置；需求真值以 docs/requirements.md v0.0.6 / R9 为准。
- R9 P1 的方向性决策（本地路由代理、opt-in 第三模式、与现有模式并存）已在需求条目确立；其实现细节、以及与项目宪法原则 II（CLAUDE_CONFIG_DIR 路由机制）的衔接修订，由 plan 阶段按宪法治理流程处理。
