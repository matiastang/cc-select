# 测试验收

> 本文从需求 [requirements.md](./requirements.md) 反推验收标准：每条需求"做对了"的判定依据。
> 这些是人工/脚手架可执行的验收用例，确保实现符合需求与架构约束（[01](./requirements-analysis.md)/[02](./architecture.md)）。

---

## 用例索引（按需求）

| 用例组 | 验收的需求 | 阶段 |
|---|---|---|
| AC1 隔离性 | R1 | MVP |
| AC2 命令等价 | R2, R3 | MVP |
| AC3 切换正确性 | R2 + [04 §1](./engineering-decisions.md#1-切换时清理上一个-provider) | MVP |
| AC4 current 语义 | [04 §3](./engineering-decisions.md#3-current-命令的语义正确性) | MVP |
| AC5 GUI 配置 | R4 | 阶段 2 |
| AC6 生效语义 | R8 / [02 §5](./architecture.md#5-配置生效语义满足-r8必须讲清) | 阶段 2 |
| AC7 PS1 可视化 | R6 | 阶段 3 |
| AC8 key 安全 | R7 | 阶段 4 |
| AC9 多 shell | R2（扩展） | 阶段 5 |

---

## AC1. 隔离性（R1）—— MVP 核心

**前提**：两个终端窗口 A、B，已配置 glm、deepseek 两个 provider。

| 步骤 | 预期 |
|---|---|
| 1. 终端 A 执行 `ccs use glm` | A 的 `claude` 走 GLM |
| 2. 终端 B 执行 `ccs use deepseek` | B 的 `claude` 走 DeepSeek |
| 3. 终端 A 再次启动 claude | 仍走 GLM，**未被 B 影响** |
| 4. 终端 B 执行 `echo $ANTHROPIC_BASE_URL` | 显示 DeepSeek 的 URL，不是 GLM 的 |

**判定**：两个终端的环境变量互不可见、互不影响。任一终端切换不改变另一终端。

---

## AC2. 命令等价性（R2, R3）

| 步骤 | 预期 |
|---|---|
| 1. 新终端执行 `cc-select use glm` | 输出 export 语句（不自动生效，符合 [03 §3](./cli-design.md#3-use-的特殊性为何它需要-shell-函数而非-alias)） |
| 2. 新终端执行 `ccs use glm` | **直接切换生效**（eval 注入） |
| 3. `ccs list` 与 `cc-select list` 输出 | 完全一致 |
| 4. `ccs current` 与 `cc-select current` 输出 | 完全一致 |

**判定**：`ccs` 是 `cc-select` 的完整等价别名，而非"只管切换"的子集。

---

## AC3. 切换正确性（R2 + 清理逻辑）

| 步骤 | 预期 |
|---|---|
| 1. `ccs use glm` 后，检查 `$CLAUDE_CONFIG_DIR` | 指向 glm 的 profile 目录 |
| 2. `ccs use claude-official`（空 provider） | `$CLAUDE_CONFIG_DIR` 被 **unset**（空值），不残留 glm 目录 |
| 3. `ccs use deepseek` | `$CLAUDE_CONFIG_DIR` 指向 deepseek 的 profile 目录，glm 配置已清理 |

**判定**：切换时上一个 provider 的 `CLAUDE_CONFIG_DIR` 被正确 unset/覆盖，无残留误导。

---

## AC4. current 语义正确性（04 §3）

| 步骤 | 预期 |
|---|---|
| 1. 终端 A `ccs use glm`，终端 B `ccs use deepseek` | 各自激活不同 provider |
| 2. 终端 A 执行 `ccs current` | 显示 `glm` |
| 3. 终端 B 执行 `ccs current` | 显示 `deepseek` |

**判定**：`current` 读的是 shell 内的 `$CC_SELECT_ACTIVE`，反映**本终端**状态，而非磁盘全局状态。

---

## AC5. GUI 配置（R4）

| 步骤 | 预期 |
|---|---|
| 1. 启动 `cc-select gui` | 打开 GUI 配置界面 |
| 2. 在 GUI 中新增/编辑/删除一个 provider | 操作成功，保存后反映到共享存储 |
| 3. 终端执行 `ccs list` | 能看到 GUI 刚改的 provider |

**判定**：GUI 能完成 provider 增删改查，且与 CLI 共享同一份配置存储。

---

## AC6. 配置生效语义（R8）

| 步骤 | 预期 |
|---|---|
| 1. 终端 A 已 `ccs use glm`，并启动 claude | 走 GLM |
| 2. 在 GUI 中修改 glm 的 URL | 保存 |
| 3. 终端 A 中**已在运行**的 claude | **不自动**切到新 URL |
| 4. 终端 A 重新 `ccs use glm` 后启动 claude | 读到新 URL |

**判定**：GUI 改配置是改"模板"，已在用的终端需重新 `ccs use` 才生效。不自动同步（这是隔离的正确语义）。

---

## AC7. PS1 可视化（R6）

| 步骤 | 预期 |
|---|---|
| 1. `ccs use glm` 后查看提示符 | 显示 `[glm]`（或配置的格式） |
| 2. 未切换任何 provider | 提示符不显示或显示默认标记 |
| 3. 用户原有 PS1 自定义 | 不被破坏（或可通过开关配置） |

---

## AC8. key 安全（R7）

| 步骤 | 预期 |
|---|---|
| 1. 查看配置存储文件 | API key **不以明文**落盘（走 Keychain/加密） |
| 2. `ccs use <provider>` 仍能正确注入 key | key 从安全存储读取后正确 export |
| 3. 退而求其次方案（明文） | 文件权限为 `600`，README 已说明风险 |

---

## AC9. 多 shell 支持（阶段 5）

| 步骤 | 预期 |
|---|---|
| 1. 在 zsh / bash / fish 中分别 `cc-select init` 并 source | 各 shell 均生成可用的 `ccs` 函数 |
| 2. 各 shell 中 `ccs use glm` | 均能切换生效 |
| 3. **Windows**：在 PowerShell 中 `cc-select init`（写入 `$PROFILE`）并重载 | 生成可用的 `ccs` 函数 |
| 4. PowerShell 中 `ccs use glm` 后 `claude` | 走对应服务商（process-scope 隔离生效） |
| 5. 两个 PowerShell 窗口分别 `ccs use` 不同 provider | 互不影响（与 zsh 行为对齐） |
| 6. CMD 中尝试 `ccs` | **不支持**（明确提示用户使用 PowerShell，见 [windows-support §4](./windows-support.md#4-为何不支持-cmd)） |

---

## 验收自动化建议

- AC1–AC4（隔离、等价、清理、current）可写成 shell 测试脚本：开两个子 shell、设置断言环境变量。
- AC5–AC6 涉及 GUI，需人工或 e2e 框架（如 Tauri 的 WebDriver）验证。
- AC7–AC9 人工验收为主。

> 每个交付阶段（见 [路线](./roadmap.md)）完成后，对照本文件对应用例组验收，全部通过方可进入下一阶段。
