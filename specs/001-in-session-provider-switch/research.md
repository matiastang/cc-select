# Research: 会话内切换 provider（Phase 0）

> 前置调研（本次对话内已完成可行性深析 + 代码 grounding：`internal/profile`、`internal/switcher`、`internal/shell`、`internal/config`、`internal/prefs`、`internal/secrets`、`internal/web/api.go`）。
> 外部事实来源：Claude Code 官方文档（env 类变量 session-init 一次性读取、settings 大多数热重载但 env 不在其中）、[anthropics/claude-code#62656](https://github.com/anthropics/claude-code/issues/62656)、claude-code-router（本地代理热切先例）。

## A. Spec 遗留决策（用户跳过 /speckit-clarify，此处取默认值，均可否决）

### D1. 会话内切换的生效范围 = 终端级（spec Q 未决项 1）

- **Decision**: 切换即更新该终端的激活 provider；之后在该终端新启动的会话沿用新 provider。
- **Rationale**: 与实体定义「一个终端任一时刻恰好一个生效 provider」吻合；避免「退出后掉回已爆限额的旧 provider」的惊讶；Mode P 下 env 恒定（TID 不变），新会话天然跟随路由表，无需额外语义。
- **Alternatives**: 仅当前会话生效（会造成 current 语义与下次启动不一致，需引入双状态）；每次带参数选择（增加交互成本，无对应需求）。

### D2. 官方 provider 不参与 Mode P 热切（v1）

- **Decision**: Mode P 仅支持第三方 provider 之间的热切；官方（`claude-official`，OAuth 订阅）继续走现有模式（直连/eval）。spec 对应 edge case 已同步修订。
- **Rationale**: 官方走 OAuth 凭据（刷新令牌、凭证文件），代理它需要 daemon 管理 OAuth 生命周期，复杂度高；限额救急场景（R9 起点）发生在第三方窗口，价值低。
- **Alternatives**: 完整支持（成本不成比例）；只支持从官方切出（仍需 OAuth 转发，同样成本）。

### D3. P0 三步工作流仅文档化，不新增组合命令

- **Decision**: P0 = acceptance-tests.md 用例 + README/isolation-modes 文档；不造 `ccs resume` 类封装。
- **Rationale**: `claude --continue` 本身已是一条命令，组合封装收益低（省一次敲击）；YAGNI（宪法 IV 简单性）。
- **Alternatives**: `cc-select use X --resume` 包装（引入 claude 进程生命周期管理，职责越界）；shell 别名建议（可作为 README 小贴士，不入产品）。

## B. 核心机制决策

### D4. 第三模式命名与挂载点：Mode P（"proxy"）

- **Decision**: `prefs.Mode` 增加 `ModeProxy Mode = "proxy"`；ResolveMode 既有三级优先级（`--mode` > provider 覆盖 > 全局 > 默认）原样兼容；`cc-select mode` / `/api/v1/mode` 接受 `proxy`。
- **Rationale**: 复用既有模式解析与存储，零新概念；"proxy" 直白表达机制。
- **Alternatives**: 独立布尔开关 `routerEnabled`（与隔离模式语义正交性差，prefs 双字段组合态难解释）；叫 "hot-switch"（描述需求而非机制）。

### D5. 身份/路由分离的具体切分（framing 的落地）

- **Decision**:
  - **恒定项进 profile settings.json**：Mode P 的 profile `env` 仅含 `ANTHROPIC_BASE_URL=http://127.0.0.1:<port>`。Mode B 的「profile env 整体替换全局 env」语义顺带屏蔽了用户全局 `~/.claude/settings.json` 里遗留 `ANTHROPIC_*` 的干扰（engineering §6 场景）。
  - **动态项进 shell env**：`ANTHROPIC_AUTH_TOKEN=$CC_SELECT_TID`（伪 token 即终端身份）。
  - **路由真值**：`~/.cc-select/routes.json`，tid → providerID；daemon 每请求重读。
- **Rationale**: claude env 冻结的变量全是恒定项（BASE_URL、TID）；会变的只有路由表。settings.json 替换语义恰好覆盖「全局 env 污染」的已知坑，无需 apiKeyHelper。
- **Alternatives**: BASE_URL 也走 shell export（会被全局 settings env 覆盖，D5 场景失效）；真 token 放 claude env（无法会话内换）。

### D6. 终端身份 TID 的生成与守卫

- **Decision**: `ccs use`（proxy 模式）发射守卫式导出：shell 里 `CC_SELECT_TID` 未设则生成一次（`ccs-` + 128bit 随机 hex），已设则跳过。实现上 `shell.Change` 增加 `OpSetIfUnset`，zsh/bash 渲染为 `if [ -z "${CC_SELECT_TID:-}" ]; then export CC_SELECT_TID='...'; fi`，PowerShell 渲染等价 `if (-not $env:CC_SELECT_TID) { ... }`。TID 随 shell 进程消亡。
- **Rationale**: 每 shell 一个稳定身份是路由归属的正确粒度；守卫保证多次 `use` 不换身份；随机 128bit 使伪 token 不可猜测。
- **Alternatives**: 用 shell PID（PID 复用会串终端，破坏隔离保证）；每次 use 新 TID（路由历史膨胀且切换后旧会话失联）。

### D7. Daemon 生命周期：按需拉起，无常驻服务安装

- **Decision**: `cc-select router serve` 为 daemon 入口（同一 binary 子命令）；`use`/`route`（proxy 模式）先 `ensureDaemon`：读 `router.json` 状态 → 探测 `/healthz` → 健康则复用，否则 detached 拉起并等 healthz ≤2s。daemon 重启**必须沿用状态文件里的 addr**（claude 的 BASE_URL 已固化在该端口上，换端口=正在运行的会话全体失联）。`router ensure|stop` 独立子命令；崩溃后任何 cc-select 命令即自愈，**无需重启 claude 会话**。
- **Rationale**: 免 launchd/systemd/计划任务（Q9 默认「按需自动就绪」）；状态文件 + healthz 握手防多实例；端口恒定支撑「恢复不重开会话」。
- **Alternatives**: 用户级常驻服务（三 OS 服务化成本高、安装态复杂）；每次 use 前台起进程（关终端即死）。

### D8. 真实 token 存储与解析（US4）

- **Decision**: 启用 Mode P（全局或 per-provider）时执行**密钥迁移**：各 provider env 中敏感值（`ANTHROPIC_AUTH_TOKEN`/`ANTHROPIC_API_KEY`）写入 keychain（service `cc-select:<id>:<var>`，既有约定），`providers.json` 原值替换为 `$keychain:` 占位符（既有机制）。daemon 收到请求时解析占位 → 真值，**内存缓存、绝不落盘**。未启用 Mode P 时行为完全不变（明文现状维持，宪法 VI 不降级）。
- **Rationale**: 复用 `internal/secrets` 既定升级路径（roadmap 阶段 6 方向）；profile settings.json 在 Mode P 下不含任何密钥，US4 直接达标。
- **Alternatives**: daemon 自有加密存储（另起炉灶，违反宪法 VI「向 keychain 收敛」）；迁移做成全局一次性（无论是否启用 Mode P 都迁——超出 R9 范围，留给阶段 6）。

### D9. model 名改写：daemon 侧、仅请求体

- **Decision**: daemon 解析请求 JSON body，若当前路由 provider 的 env 定义了 `ANTHROPIC_MODEL`，则把 body 的 `model` 字段替换为该值（所有出现的 model 统一映射到主模型）；未定义则透传。响应与 SSE 流**不动**。
- **Rationale**: 会话内切换后 claude 仍按启动时认知发送旧 model id，改写必须在代理侧；「全部映射到主模型」是 v1 已知简化（后台小任务也用主模型）。
- **Alternatives**: 精细映射（区分 main/background model——daemon 无法可靠区分 body 中 model 的语义角色，留作后续按 header/路径启发式增强）；profile 里设 `ANTHROPIC_MODEL`（launch 时正确，切换后即错）。

### D10. 路由表读写与并发

- **Decision**: `routes.json`（0600、原子写，同 providers.json 模式）；daemon **每请求重读**（文件极小，免去三 OS 文件监听）；CLI 写入前校验 provider 存在。
- **Rationale**: 简单正确；写频极低（每次切换），读频每请求但 <1KB。
- **Alternatives**: fsnotify/inotify 监听（三 OS 行为差异 + 新依赖）；daemon 内存表 + 管理 API（引入 CLI↔daemon 通信协议，复杂化）。

### D11. 端口与配置

- **Decision**: 默认 `127.0.0.1:48270`；`CC_SELECT_PROXY_ADDR` 可覆盖（写入 router.json，daemon 优先读状态文件）。默认端口被**非本 daemon**进程占用 → 报错并提示用 env 换端口（不静默换随机端口，避免与用户预期不符）。
- **Rationale**: 恒定端口是「崩溃自愈不换址」的前提；显式冲突优于隐式漂移。
- **Alternatives**: 随机端口写状态文件（首次可用，但用户排障/防火墙场景体验差）。

### D12. 宪法修订（治理动作，随实现落地）

- **Decision**: 原则 II 修订 1.1.0 → **1.2.0**（MINOR）：增补 Mode P 条款——「opt-in 代理路由模式下，provider 路由经本地 daemon 路由表；env 仅承载恒定身份（BASE_URL=router、AUTH_TOKEN=TID）；`current` 在 TID 存在时以路由表为 per-terminal 真值」。附 Sync Impact Report，来源 docs/requirements.md R9。
- **Rationale**: 宪法 VII/Governance：docs/ 赢，宪法随后修订；MINOR（新增豁免条款，不删不改既有原则语义）。

### D13. 隔离与切换的自动化验证（SC-003 脚本化）

- **Decision**: 集成测试用 `httptest` 起 **fake upstream**（记录每笔请求的 model、Authorization、路径），起真实 daemon 于随机端口；模拟两个 TID 的 claude 客户端交叉切换 ≥10 次，断言每笔请求按各自路由表当前值转发——SC-003 从人工演练升级为可回归的自动化用例。
- **Rationale**: 真 provider 限额不可编程模拟；fake upstream 让「路由正确性」与「上游质量」解耦。
- **Alternatives**: 只做真机手工验收（不可回归）；mock 整个 daemon（测不到转发/改写真实路径）。

## C. 已知限制（v1 明示，非缺陷）

- **L1**：`~/.claude/settings.json` 全局 env 若定义 `ANTHROPIC_AUTH_TOKEN`，且用户**不经 ccs use** 裸启 claude（无 TID）→ daemon 返回 401 并在响应体给出指引（`ccs use` 或清理全局 env）。经 `ccs use` 启动则被 profile env 整体替换语义屏蔽（D5）。
- **L2**：daemon 未运行时的首批请求表现为连接拒绝；任一 cc-select 命令触发自愈后**后续**请求恢复（进行中那笔由 claude 自身重试语义处理）。
- **L3**：D9 的 model 统一映射简化（后台任务用主模型）。
- **L4**：TID 随 shell 消亡后路由条目成孤儿 → `cc-select route prune`（按 updatedAt 过期清理）手工收敛；v1 不做自动 GC。
