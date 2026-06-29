# 需求分析

> 本文回答：需求 [R1](./requirements.md#r1-shell-级模型服务商隔离) 能不能做、为什么能做、有哪些不可违背的约束、与 cc-switch 的本质差异。
> 需求源头见 [requirements.md](./requirements.md)。

---

## 1. 可行性结论

**完全可行，且技术路径清晰。**

核心依据：环境变量天然是**进程级 / shell 级**的。父 shell `export` 的变量只被子进程继承，**绝不能反向影响其他终端**。

把 cc-switch"写全局配置文件"的动作，替换成"在当前 shell 内 export 环境变量"，即可获得 shell 级隔离。

这是 `nvm`、`pyenv`、`direnv` 等工具共同采用的标准模式。

---

## 2. 与 cc-switch 的对比（满足 R1、R5）

| | cc-switch | cc-select |
|---|---|---|
| 作用对象 | 写 `~/.claude/settings.json`（磁盘文件） | 在当前 shell 内 `export CLAUDE_CONFIG_DIR` 指向独立 profile 目录（内存中的环境变量） |
| 作用范围 | 全机器所有终端 | 仅当前终端及其子进程 |
| 生效方式 | 改文件，下次启动 claude 时读取 | shell 内 export，立即生效 |
| 切换副作用 | 影响所有其他终端 | 零影响其他终端 |
| 语义 | switch（全局切换） | select（按会话选择） |

两者**互补而非替代**：
- cc-switch 适合"现在我想统一换一个默认服务商"；
- cc-select 适合"我要同时跑多个服务商做对比 / 不同项目用不同服务商"。

---

## 3. 核心架构约束（动手前必读）

整个项目建立在一个 Unix 事实及其推论之上：

- **事实**：子进程**无法**修改其父 shell 的环境变量。无论 CLI 二进制做什么，它都无法把 `export` 注射回启动它的 shell。
- **推论**：`cc-select` 必须拆成**两个协作层**——
  1. 一个二进制，负责**输出** shell 语句（`export` / `unset`）；
  2. 一个 shell 包装函数（装到 `~/.zshrc` / `~/.bashrc`），负责在调用方自己的 shell 里 `eval` 这些语句。

> **任何试图"单纯靠二进制切换 provider"的设计，在设计层面就是错的。**

这条约束直接决定了后续架构（见 [架构设计](./architecture.md)）和 CLI 设计中 `use` 命令的特殊处理（见 [CLI 设计](./cli-design.md#3-use-的特殊性为何它需要-shell-函数而非-alias)）。

---

## 4. 需求覆盖矩阵

每条需求在哪个文档被实现/解决：

| 需求 | 解决于 |
|---|---|
| R1 Shell 级隔离 | [架构设计](./architecture.md)（eval 隔离）+ [验收](./acceptance-tests.md)（隔离用例） |
| R2 命令行切换 | [CLI 设计](./cli-design.md)（`use` 命令） |
| R3 `ccs` 别名 | [CLI 设计](./cli-design.md#1-命令总览cc-select-是主命令ccs-是其短别名) |
| R4 GUI 配置 | [架构设计](./architecture.md#4-gui-配置界面) + [技术选型](./tech-stack.md) |
| R5 安装体验 | [分发与安装](./distribution.md) |
| R6 PS1 可视化 | [工程细节](./engineering-decisions.md) |
| R7 key 安全 | [工程细节](./engineering-decisions.md) + [技术选型](./tech-stack.md)（keychain 占位已预留，当前为明文存储） |
| R8 生效语义 | [架构设计](./architecture.md#5-配置生效语义满足-r8必须讲清) |
