# 交付路线

> 本文给出从简到强的交付阶段规划。每个阶段标注对应的详细文档与验收点。
> 上游：需求全集见 [requirements.md](./requirements.md)，验收见 [acceptance-tests.md](./acceptance-tests.md)。
> 决策已更新：GUI=Web、存储=JSON、shell 先 zsh（可扩展）、OS 跨三平台（见 [tech-stack](./tech-stack.md#0-决策总览)）。

---

## 阶段规划（从简到强）

| 阶段 | 内容 | 主要满足需求 | 详细文档 | 验收 | 状态 |
|---|---|---|---|---|---|
| **1. MVP（CLI 切换 + Web 配置）** | CLI 二进制（`cc-select`）+ JSON 存储 + `ccs` 别名/shell 函数 + `use/list/current/init` + **本地 Web 配置页**（`cc-select gui`，增删改查 provider）。配置同时支持命令行 `add` 与 Web 两种入口。 | R1, R2, R3, R4 | [cli-design](./cli-design.md)、[architecture](./architecture.md) | [验收 - 隔离/等价/配置](./acceptance-tests.md) | ✅ 已完成 |
| **2. key 安全** | key 迁入系统 Keychain（[R7](./requirements.md#隐含-派生需求)）。 | R7 | [engineering §5](./engineering-decisions.md#5-api-key-安全满足-r7) | [验收 - 安全](./acceptance-tests.md) | 🟡 部分完成：keychain 抽象与占位机制已实现，但 CLI/Web 写入路径仍为明文 |
| **3. PS1 集成** | `init` 自动注入提示符 hook，显示当前 provider。 | R6 | [engineering §4](./engineering-decisions.md#4-提示符可视化满足-r6) | [验收 - 可视化](./acceptance-tests.md) | ⏳ 未开始 |
| **4. 跨 shell** | 在 zsh 基础上扩展 bash / fish 的 wrapper（init 按 shell 分发）。 | R2（扩展） | [cli-design](./cli-design.md) | [验收 - 多 shell](./acceptance-tests.md) | 🟡 部分完成：zsh/bash 共用 emitter、PowerShell emitter 已实现；fish 未支持 |
| **5. 跨平台完善** | macOS/Linux/Windows 全覆盖；尤其 Windows 的环境变量隔离机制单独评估。 | Q6 | [architecture §2.1](./architecture.md#21-跨平台约束满足-q6macoslinuxwindows) | [验收 - 多 shell](./acceptance-tests.md) | 🟡 部分完成：PowerShell emitter、`$PROFILE` 写入（含 BOM/加载）与 CI 集成测试已实现；fish 未支持 |

---

## MVP 细化（阶段 1）

MVP 是验证整个方案地基的关键里程碑。因用户希望"有地方能更新配置"，MVP **同时含命令行切换与 Web 配置**（二者读写同一份 JSON）：

1. **切换**：`cc-select use <provider>` 输出正确的 `export CLAUDE_CONFIG_DIR` / `unset CLAUDE_CONFIG_DIR` 语句（含 `CC_SELECT_ACTIVE`）。
2. **shell 集成**：`cc-select init` 输出可用的 `ccs` 别名 + shell 函数代码（zsh/bash/PowerShell）。
3. **命令等价**：`ccs use glm` 与 `cc-select use glm` 行为等价（[R3](./requirements.md#r3-cc-select-与-ccs-两个命令都要能用ccs-是-cc-select-的别名)）。
4. **隔离**：两个终端可独立切换、互不影响（[R1](./requirements.md#r1-shell-级模型服务商隔离)）。
5. **存储**：读写 JSON（`providers.json` 存 id/name；`profiles/<id>/settings.json` 存 env）。
6. **Web 配置页**（[R4](./requirements.md#r4-图形化界面配置服务商)）：`cc-select gui` 起本地服务，浏览器内对 provider 增删改查，保存到同一份配置。

> MVP 已完成。当前剩余工作见上方阶段 2–5。已知问题"与 cc-switch 共存冲突"见 [engineering §6](./engineering-decisions.md#6-claude-的-settingsjson-env-优先级高于-shell已用-claude_config_dir-解决)。
