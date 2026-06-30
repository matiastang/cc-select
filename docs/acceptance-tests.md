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
| AC10 隔离粒度 | [isolation-modes.md](./isolation-modes.md) / [04 §7](./engineering-decisions.md#7-隔离粒度全隔离-vs-仅-settingsjson-隔离双模式) | 后 MVP |

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
| 4. GUI 顶部「全局隔离模式」选择器 | 可切换 `settings-only` / `full`，保存后写入 `~/.cc-select/prefs.json` |
| 5. 编辑 provider 时「隔离模式」选择器 | 可选「继承全局 / 仅 settings.json 隔离 / 整目录隔离」，保存后写入 `providers.json` |
| 6. 重新打开 GUI 或切换编辑/列表 | 全局模式和每个 provider 的模式回显为上次保存的值 |

**判定**：GUI 能完成 provider 增删改查，能设置并回显全局与 per-provider 隔离模式，且与 CLI 共享同一份配置存储。

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

## AC10. 隔离粒度（双模式）—— Mode A / Mode B

> 设计见 [isolation-modes.md](./isolation-modes.md)；机制见 [工程细节 §7](./engineering-decisions.md#7-隔离粒度全隔离-vs-仅-settingsjson-隔离双模式)。

| 步骤 | 预期 |
|---|---|
| 1. `cc-select mode`（未设置过） | 输出 `settings-only`，提示「未显式设置，使用默认」 |
| 2. 用户已有 `~/.claude`（含 permissions 的 settings.json + projects/）；`cc-select add glm` 后 `ccs use glm` | profile `settings.json` 含 provider env **且保留全局** permissions/model；`projects/` 等为指向 `~/.claude/` 的软链（Mode B 共享） |
| 3. 经 profile 软链写入 `projects/x` | 内容落到共享的 `~/.claude/projects/x`（共享生效） |
| 4. 全局 `cc-select mode full` 后 `ccs use glm` | profile 目录回到只剩 `settings.json`（软链被清理，真隔离） |
| 5. 全局 full，但 `cc-select edit glm --mode settings-only` 后 `ccs use glm` | glm 仍为共享（per-provider 覆盖胜过全局） |
| 6. `ccs use glm --mode full`（一次性，不落盘） | 本次全隔离；`cc-select mode` 全局值不变 |
| 7. `cc-select edit glm --mode default` | 清除 glm 的 per-provider 覆盖（继承全局） |
| 8. 从未装 claude（无 `~/.claude`）时 `ccs use glm` | 不报错；profile 仅 settings.json（无东西可共享），后续产生 `~/.claude` 内容后下次 use 自愈共享 |
| 9. 全局或 provider 模式为 B 时 `ccs use X` 改全局 settings.json/装插件后再次 use | 自愈：profile 自动反映最新全局 + 链接修复 |
| 10. Windows：目录型条目用 junction（免特权）共享；文件型无开发者模式则跳过+告警 | 不强制退化为 Mode A；不报错中断 |
| 11. Web GUI 中 provider 设为 Mode A（full）后保存含 `permissions`/`model` 的 settings.json | 这些非 env 字段原样持久化到 profile settings.json |
| 12. Web GUI 中 provider 设为 Mode B（settings-only）后保存含 `permissions`/`model` 的 settings.json | 仅 `env` 被持久化到 profile；非 env 字段来自全局 `~/.claude/settings.json`（Mode B 语义） |

### 已知前置验证（动手前/发版前）

- `~/.claude.json`（sibling 大状态文件）是否被 `CLAUDE_CONFIG_DIR` 重定位——决定是否需纳入共享。
- 设 `CLAUDE_CONFIG_DIR` 后 claude 是否完全不读全局 `~/.claude/settings.json`——印证合并必要性。
- 详见 [isolation-modes.md §6](./isolation-modes.md#6-待验证项动手前用真机确认)。

---

## 验收自动化建议

- AC1–AC4（隔离、等价、清理、current）可写成 shell 测试脚本：开两个子 shell、设置断言环境变量。
- AC5–AC6 涉及 GUI，需人工或 e2e 框架（如 Tauri 的 WebDriver）验证。
- AC7–AC9 人工验收为主。

> 每个交付阶段（见 [路线](./roadmap.md)）完成后，对照本文件对应用例组验收，全部通过方可进入下一阶段。
