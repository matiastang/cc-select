# 需求

> 本文档是**用户原始诉求的唯一记录源**，如实整理用户提出的要求，不改写诉求本身。
> 后续所有文档（分析、架构、设计、验收）均从本文档推导而来。
> 如需求变更，**先改这里**，再沿影响链更新下游文档（见 [README](./README.md#更新协作规范重要)）。

---

## 核心诉求

> 现在使用 Claude Code 的时候，可以用 cc-switch 来切换不同的模型，但它是**全局的**——切换后所有命令行里都变了。
> 希望实现：**同一台电脑上，不同的 shell（终端窗口）里能使用不同服务商的模型**，而不是一换全换。

### 背景参照

- 现有工具 [cc-switch](https://github.com/farion1231/cc-switch) 切换模型服务商是**全局**的（改 `~/.claude/settings.json`，全机器共享）。
- 目标工具 `cc-select` 要做到 **shell 级（per-terminal）隔离**。

---

## 明确的需求条目

### R1. Shell 级模型服务商隔离

- 同一台机器上，不同终端窗口可同时使用不同服务商的模型。
- 在某个终端里切换，**只影响该终端及其子进程**，不影响其他终端。

→ 可行性见 [需求分析](./requirements-analysis.md)，架构见 [架构设计](./architecture.md)。

### R2. 命令行切换（`ccs`）

- 提供命令行方式切换当前终端的 provider。
- 切换命令用起来要简单（如终端敲一行）。

→ CLI 设计见 [CLI 设计](./cli-design.md)。

### R3. `cc-select` 与 `ccs` 两个命令都要能用，`ccs` 是 `cc-select` 的别名

> 用户原话：希望 `cc-select` 和 `ccs` 两个指令都能使用，`ccs` 只是 `cc-select` 的一个别名。

- `ccs` 是 `cc-select` 的**短别名**，二者功能完全等价（不是"ccs 只管切换"的子集）。

→ 别名机制见 [CLI 设计 - 命令总览](./cli-design.md)。

### R4. 图形化界面配置服务商

> 用户原话：配置的话要是可以像 cc-switch 一样，有图形化的界面来配置就更好了。

- **配置 provider**（填 URL / key / model）走 GUI，对标 cc-switch 的体验。
- 切换仍走命令行（`ccs`），两者分工：GUI 配置、命令行切换。
- **GUI 形态已定为本地 Web 网页**（`cc-select gui` 起本地 HTTP 服务、浏览器打开）。理由：轻量、跨平台天然、与 CLI 实现语言解耦。**桌面 App（Tauri/Electron 等）作为备选保留**，日后可切换（见 Q2）。

→ GUI 形态见 [架构设计 - GUI 配置界面](./architecture.md#4-gui-配置界面)，分发见 [分发与安装](./distribution.md)。

### R5. 安装体验对标 cc-switch

> 用户原话：我记得命令行装了 cc-switch 并没有手动安装应用，自动就装了 App，这个怎么实现的？我们的方案能实现吗？

- 期望 `cc-select` 也能"命令行一键安装"。
- 因 GUI 选 Web 路线，安装方式相应变化：CLI 走包管理器/npm/单二进制，Web GUI 随 `cc-select gui` 命令启动，**无需安装桌面 App**。
- 桌面 App + Homebrew Cask 那套（cc-switch 的方式）作为**备选保留**，若日后 GUI 改回桌面 App 可启用。

→ 安装方案见 [分发与安装](./distribution.md)。

---

## 隐含 / 派生需求

下列由核心诉求推导得出，需与用户确认是否纳入：

- **R6（派生）状态可视化**：多终端并行时，用户需知道当前终端用的是哪个 provider（PS1 显示）。
- **R7（派生）安全**：API key 的存储需考虑安全（至少 README 说明风险，理想情况接入系统 Keychain）。
- **R8（派生）配置生效语义**：GUI 改了配置后，已运行的终端是否自动同步——这是 shell 级隔离的必然推论，需在文档中讲清并让用户知晓。

---

## 待用户确认的开放问题（含已定决策）

| 编号 | 问题 | 决策 / 当前倾向 |
|---|---|---|
| Q1 | 语言选型（Rust / Node / Go / 脚本） | **已定：Go**。理由：单二进制（用户无需装运行时，对 Windows 友好）、跨三平台交叉编译最省心、启动快（CLI 频繁调用）、标准库即可起 Web 配置页。Node+TS / Rust 作为备选保留。 |
| Q2 | GUI 形态（桌面 App / 本地 Web 服务） | **已定：本地 Web 服务**。桌面 App（Tauri/Electron/Wails）作为备选保留。 |
| Q3 | 存储格式（JSON / SQLite / 多文件） | **已定：JSON**（原子写）+ key 走 Keychain。SQLite/多文件作为备选保留。 |
| Q4 | 派生需求 R6/R7/R8 是否纳入 | **已纳入**（见上文 R6/R7/R8，验收见 [acceptance-tests.md](./acceptance-tests.md)）。 |
| Q5 | 目标 shell 范围 | **已定：zsh / bash / PowerShell**；fish 作为后续扩展。`init` 与 shell 发射器按 shell 类型可扩展设计。 |
| Q6 | 目标 OS | **已定：macOS、Linux、Windows 三平台都要能跑**。影响：环境变量机制、shell 函数、打包产物需覆盖三平台。 |

> 备选方案保留在 [技术选型](./tech-stack.md)，日后调整实现方案时可回看。

---

## 变更记录

| 日期 | 变更 | 来源 |
|---|---|---|
| 2026-06-26 | 初始整理：从多轮需求讨论提炼 R1–R5 + 派生 R6–R8 | 首次创建 |
| 2026-06-26 | 记录 Q2–Q6 决策：GUI=本地Web、存储=JSON、shell=zsh(可扩展)、OS=三平台、R6–R8 纳入；备选方案保留 | 用户确认 |
| 2026-06-26 | Q1 决策：语言定为 **Go**（Node+TS/Rust 备选）。Windows 支持评估完成（可行，仅不支持 CMD）。 | 用户确认 |
| 2026-06-28 | 实测发现"与 cc-switch 共存"冲突：变量名不一致导致并存而非覆盖。记为已知问题，见 [工程细节 §6](./engineering-decisions.md)。短期建议先清 settings.json 的 env。 | 实测发现 |
| 2026-06-28 | **重大发现**：实测确认 claude 的 settings.json `env` 优先级**高于** shell 环境变量。cc-select 的 shell 隔离机制在"装过 cc-switch（settings.json 有 env）的机器"上**对 claude 完全失效**。推翻此前"同名覆盖"假设。需重新评估方向（多 CLAUDE_CONFIG_DIR 等），见 [工程细节 §6](./engineering-decisions.md)。 | 实测发现 |
| 2026-06-28 | **机制重构落地**：改用 `CLAUDE_CONFIG_DIR`（方向 2，已实测验证）。`ccs use X` 指向 `~/.cc-select/profiles/<id>/`，claude 读该目录 settings.json。token 明文落 profile（keychain 占位机制已预留待接入）；官方 provider = unset 回默认。详见 [架构 §2.0](./architecture.md)、[工程细节 §6](./engineering-decisions.md)。 |
| 2026-06-29 | **文档与实现同步**：更新 CLAUDE.md、docs 状态概览与路线图；统一 docs 与代码中的环境变量名为 `ANTHROPIC_AUTH_TOKEN`；修正 CLI/Windows/验收用例中 `CLAUDE_CONFIG_DIR` 相关示例。 | 文档整理 |

