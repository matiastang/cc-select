# Research: model-picker-sync 决策记录

> Phase 0 产物。所有 NEEDS CLARIFICATION 已消解——代码事实（本仓库）、平台官方文档、2026-08-29 三轮探针实验三方证据。
> 证据分级：【代码】= 本仓库现状；【实验】= 2026-08-29 真机探针实验；【文档】= Claude Code 官方文档；【001】= specs/001 既有研究结论。

## D1. 模型清单来源：env 四变量派生（零 schema 变更）

- **决策**：`ModelPlan` 由 `Provider.Env` 纯函数派生（Main/Opus/Sonnet/Haiku 四槽 + 去重 Entries）；不新增 provider `models` 字段。
- **理由**：requirements Q11 已确认默认倾向；provider 显式 models 字段（任意多模型 + 自定义 label）留作后续增强，YAGNI（宪法技术约束）。`Provider` 结构现状【代码：`internal/config/config.go:51`】无 models 字段，新增属 schema 变更，超出本期范围。
- **备选**：provider `models` 字段——拒绝因本期非目标；若未来 GLM 单 provider 需暴露 >4 模型时自然引入。

## D2. 注入点：profile 构建唯一咽喉（mergeSettings 后置步）

- **决策**：注入作为 `syncSettingsOnly` 合并路径的收尾步（`mergeSettings` 输出上叠加 modelPicker / Mode P 的 model）；Mode A 的 `syncFull` 同样叠加。`SyncProxy`（Mode P）改造为**同时传入 providers.json 的 `Provider.Env`** 供派生——而非 profile env（profile 只有代理 BASE_URL）【代码：`internal/profile/build.go:82-92`】。
- **理由**：add/edit/use/web 全部经 `Sync`/`SyncProxy` 一个入口【代码：`build.go:12-16`】，单点注入天然覆盖所有写路径且幂等；Mode P 真值源上抬与 `upliftLegacyEnv` 的既有评审结论 #1 一致（providers.json 才是 env 真值）。
- **备选**：在各调用方（add/edit/use/web）分别注入——拒绝，四个写路径必然漂移。

## D3. 热切刷新路径：`$CLAUDE_CONFIG_DIR` 继承 + 写发射 profile（关键正确性决策）

- **决策**：`route switch` 成功后读取进程 env `$CLAUDE_CONFIG_DIR`，对其下 settings.json 做读-改-写原子刷新；env 未设 → 静默跳过。
- **正确性核心**：写入目标是**发射时的 profile**（`$CLAUDE_CONFIG_DIR` 指向的目录），**不是** `profiles/<新 provider>/`。热切 glm→minimax 后 `CLAUDE_CONFIG_DIR` 仍指 `profiles/glm/`【001：身份/路由分离设计——env 恒定】；写 minimax 的 profile 运行中会话毫无感知。这正是 requirements Q13 倾向 env 路径的深层原因，也排除了「经 routes.json 的 tid→provider 反查 profile」方案（反查到的是**路由目标** provider，热切场景下写错文件）。
- **可达性证据**：claude 的 Bash/`!` 子进程继承会话 env——001 研究 D6 已确立（`CC_SELECT_TID` 经同一信道解析，`internal/cli/route.go:78` 注释明示），`CLAUDE_CONFIG_DIR` 由 `ccs()` wrapper 在发射时 export，同属会话 env，继承机制相同。三平台差异无涉（纯 env 继承语义，非 shell 特性）。
- **防护**：文件不存在 / JSON 损坏 / 不可写 → 路由切换照常成功（routes.json 是 Mode P 真值），仅 stderr 告警；不写 `~/.cc-select/profiles/` 之外的陌生目录除非 settings.json 已存在（识别为合法 profile）。

## D4. 代理改写：清单透传 → 槽位子串映射 → 主模型回落

- **决策**：R1~R6 算法见 [contracts/proxy-model-routing.md](./contracts/proxy-model-routing.md)。核心排序：**清单精确匹配先于子串分类**（防 provider id 巧合含 "sonnet" 误判）；未知 id 回落主模型（= v1 行为，热切后旧 id 自然消化）；无 `ANTHROPIC_MODEL` 全路径透传（v1 逐字节兼容）。
- **理由**：CC 在自定义 BASE_URL 下发出的 model 形态有限且已知【文档：任意 id 透传、无 recognized-id 校验】：完整目录 id（`claude-opus-*`）、别名（`opus`/`opus[1m]`）、后台 haiku id——子串分类完备覆盖。无需维护 claude id 穷举表（版本漂移维护负担）。
- **v1 兼容证明**：v1 = 「无条件主模型」；v2 在「未配置主模型」时逐字节一致（R6），在「已配置」时仅新增「清单内透传」分支——此前该分支不存在是因为清单不存在（显示与改写同源后才可能出现）。`modelrewrite.go` 既有防护（32 MiB 上限、非 JSON 透传、UseNumber）原样保留【代码】。

## D5. `availableModels` 白名单共存（FR-006）

- **决策**：注入时若检测到 settings.json 已含 `availableModels`，把注入清单中缺失的 id 追加进白名单。
- **理由**：【文档：settings-reference / model-config，2026-08-29 复核】「An availableModels allowlist still applies to these rows」——白名单外的 modelPicker 行被 **Dropped（隐藏，非 grayed）**；若无一存活，CC 明确回落内置目录（「No row survives: Claude Code keeps the built-in lineup」）。不追加 = FR-001/006 在配了白名单的用户处静默失效。
- **条目格式**【文档】：纯字符串数组（如 `["sonnet","haiku"]`），可为家族别名（`opus`/`sonnet`/`haiku`/`fable`，家族别名起家族匹配作用）、版本前缀或完整 id；**无对象、无 `*` 通配语法**。家族别名**不会**匹配 `glm-*` 类 provider id——因此追加精确的 provider id 是唯一正确做法，与既有家族条目无冲突语义。
- **频次**：白名单通常继承自全局 settings.json（mergeSettings 未知字段保留【代码：`merge.go:19`】），属小众配置；追加逻辑保持幂等（存在即跳过）。

## D6. 旧版 Claude Code 兼容（FR-007 的简化实现）

- **决策**：**不硬门控**——任何版本都注入 `modelPicker`；未知 settings 键被 CC 安全忽略（无报错、无剥离），旧版本行为与今天完全一致。检测到 `claude --version` < 2.1.242（best-effort，PATH 无 claude 则跳过）时在 `use` 输出一行升级提示。
- **理由**：【文档 2026-08-29 复核】官方发布 schema 顶层 `additionalProperties: true`（未知键连「schema 拒绝值」路径都触发不了）；「Claude Code ignores surface names it doesn't recognize rather than failing the settings file, so you can add new surface names before every client has updated」是显式前向兼容策略。直接证据：【实验】本轮探针期间运行中的 2.1.220 会话在 settings.json 含 `modelPicker` 时正常工作；且 2.1.251 会话在 CC 自身写入（`/model` 确认落 `model` 字段）后 modelPicker 行保持完整——CC 写文件时保留未知键在现行版本实证成立。
- **残留不确定**（低危）：旧版 CC 写文件时是否剥离未知键无文档承诺；即便剥离，后果仅是「选择器显示回落现状」，与注入前一致，无功能损伤。无需为此加门控。
- **best-effort 检测**：`claude --version` 输出形如 `2.1.251 (Claude Code)`，正则取 semver 比较；超时/失败静默跳过，绝不影响 `use` 主流程。

## D7. 不注入 label/description

- **决策**：modelPicker 行只含 `model`。
- **理由**：【实验】CC 对无 label 的行回退显示原始模型 id（如 `glm-5.3[1m]`）——真实、零歧义、零派生逻辑。自定义 label（如 "GLM 5.3 (1M context)"）需要 id→名称映射表，属 provider `models` 字段增强（D1）的伴生产物，本期不做。
- **收益确认**：【实验】横幅与 Default 行 "currently" 在无 label 时显示原始 id，满足 FR-002。

## D8. Mode P 的 `model` 字段注入（US1 验收场景 2 的关键补丁）

- **决策**：Mode P 构建 profile 时注入 `model=<主模型 id>`；Mode A/B 不注入。
- **理由**：Mode P 的 profile env 无 `ANTHROPIC_MODEL`，CC 启动时解析 `model` 设置定位当前模型——全局继承的 `model: opus[1m]`【实验：用户机器全局 settings.json 实况】会让 ✔ 与横幅停在目录别名上，实际服务的是主模型，显示仍假。注入 `model=<主模型>` 后 CC 内部认知 = 真实主模型，✔/横幅/请求体三者一致（发出的 id ∈ 清单 → R1 透传，闭环）。Mode B 由 env 优先级保证（`ANTHROPIC_MODEL` > `model` 设置【文档】），无需注入。
- **已知限制**：`model` 是 CC 启动期键（不热重载【文档】）→ 热切刷新后本会话「当前模型」标记保持旧值，直至用户在 `/model` 选择或重启；列表热重载不受影响（【实验】三轮探针实证）。

## D9. 文档回流义务（宪法 VII 触发项）

- 实现验收后 MUST 回流：① `docs/acceptance-tests.md` 新增 AC17（五场景对齐 quickstart.md）；② `docs/engineering-decisions.md` §8「claude 的 env 在会话启动时冻结」表述修正——官方现行文档确认 settings 文件（含 env 类变更）大多热重载，不热重载键仅 `model`/`effortLevel`/`outputStyle`【文档】（001 立项前提之一已被部分推翻，属事实修正而非推翻 Mode P 架构：Mode P 的存在依据还有「子进程不可改父 env」与「路由表真值」两条独立支柱）；③ `docs/isolation-modes.md` Mode P 节补「模型显示与切换」小节；④ README「已知限制」更新（model 不热重载的显示滞后）。
