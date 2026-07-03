# cc-select 文档集

本目录是 `cc-select` 项目的**完整需求与设计文档集合**，按软件开发生命周期组织成树形结构：从需求出发，经需求分析、架构、设计，到实现路线与测试验收。

每篇文档**只聚焦一个方向**，文档之间通过相对链接互相跳转。所有文档合起来构成项目的完整需求与方案。

---

## 设计原则：文件名只表达"是什么"，顺序只由本文件维护

> **文件名不带数字序号**（不用 `00-`、`01-` 前缀）。
> 原因：若把"阅读顺序"编码进文件名，中间插入新环节就要给后续所有文件重命名，并连带修改全部引用链接，代价巨大且污染 git 历史。
>
> 因此：
> - **文件名 = 稳定的语义标识**（它"是什么"），永不因顺序调整而改名。
> - **阅读顺序 = 由本 README 的「阅读顺序」列表单一维护**（它"排第几"）。
> - 想插入新环节、调整顺序，**只改本 README 一处**，文件名与所有链接纹丝不动。

---

## 阅读顺序

按软件开发生命周期排列。**插入新文档或调整顺序时，只改下面这个列表。**

| # | 文档 | 一句话职责 |
|---|---|---|
| 1 | [requirements.md](./requirements.md) | **用户输入的唯一源头**：原始诉求是什么，不被改写。 |
| 2 | [requirements-analysis.md](./requirements-analysis.md) | 能不能做、为什么能做、核心约束、与 cc-switch 的差异。 |
| 3 | [architecture.md](./architecture.md) | 整体怎么搭：GUI 配置 + CLI 切换的双形态、eval 两层结构、`CLAUDE_CONFIG_DIR` 数据模型、配置生效语义。 |
| 4 | [windows-support.md](./windows-support.md) | Windows 可行性评估：PowerShell 的 process-scope 隔离等价于 `export`，仅不支持 CMD。 |
| 5 | [cli-design.md](./cli-design.md) | 命令行方向的细化：`cc-select` / `ccs` 别名、子命令、`use` 为何特殊。 |
| 6 | [engineering-decisions.md](./engineering-decisions.md) | 横切的工程细节与正确性保证。 |
| 7 | [tech-stack.md](./tech-stack.md) | 选型决策与待定项汇总（语言、GUI 框架、存储格式）。 |
| 8 | [distribution.md](./distribution.md) | 怎么装到用户机器上。 |
| 9 | [release.md](./release.md) | 维护者如何发版、配置 Token、验证包管理器 manifest。 |
| 10 | [roadmap.md](./roadmap.md) | 按什么顺序交付。 |
| 11 | [acceptance-tests.md](./acceptance-tests.md) | 怎么验证"做对了"。 |

---

## 更新协作规范（重要）

> **核心原则：只有 `requirements.md` 是用户输入。其余文档都是对需求的层层推导。**

每次更新遵循这条规则：

1. **需求变更** → 先改 [requirements.md](./requirements.md)（这是唯一允许直接根据用户诉求编辑的文档）。
2. **沿影响链向下更新** → 顺次检查并更新受影响的分析/架构/设计文档，确保推导链不断裂：
   - 需求 → 分析 → 架构 → 设计（CLI / 工程细节）→ 验收
   - 选型相关变更 → 技术选型
   - 安装相关变更 → 分发
   - 阶段规划变更 → 路线
3. **不反向改写需求** → 即使方案调整，也不要为了让需求"配合方案"去改 `requirements.md`；需求如实记录用户说了什么。
4. **保持链接有效** → 文档间用相对路径（`./<name>.md`）跳转；**新建文档时只取语义名（不带序号）**，并把它加进上方「阅读顺序」列表。
5. **待定项集中标注** → 未决的选型用「待定」明确标注（多见于技术选型），不要写成既定结论。

### 文档间引用约定

- 引用其他文档：`见 [架构设计](./architecture.md#3-数据模型元信息索引-profile-真值两层)`。
- 同一文档内：用标准 Markdown 锚点。
- 跨文档传递结论时，**在源头文档写完整推导，在目标文档只放结论 + 源头链接**，避免同一份推导散落多处导致不一致。

---

## 当前状态速览

- **项目阶段**：MVP 实现已完成，进入迭代完善期（见 [roadmap.md](./roadmap.md)）。
- **已确定选型**：Go、本地 Web 服务 GUI、JSON 存储 + profile 目录、`CLAUDE_CONFIG_DIR` 切换机制、zsh/bash/PowerShell 支持（见 [tech-stack.md](./tech-stack.md)）。
- **剩余主要工作**：PS1 提示符集成、fish shell 支持、API key 存储方式统一（keychain 占位 vs 明文）。Windows PowerShell 集成已由 CI 覆盖。
- **本文档集由** `technical-design.md`（单篇长文档）拆分而来；旧文件已删除。
