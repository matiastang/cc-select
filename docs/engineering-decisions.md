# 工程细节

> 本文记录横切的工程细节与正确性保证——这些是"做对"而非"做出来"的关键点。
> 上游：需求 [R6](./requirements.md#隐含-派生需求)/[R7](./requirements.md#隐含-派生需求)，架构见 [架构设计](./architecture.md)。
> 注：存储格式选型见 [技术选型](./tech-stack.md#4-存储格式json-两层元信息-profile-真值q3)。

---

## 1. 切换时清理上一个 provider

从 provider A 切到 B，必须确保 B 的配置生效、A 的配置不残留。

当前机制只操作两个变量：
- `CLAUDE_CONFIG_DIR`：普通 provider 设为对应 profile 目录，官方 provider 则 `unset`；
- `CC_SELECT_ACTIVE`：始终设为当前 provider ID。

由于 claude 启动时读取的是 `CLAUDE_CONFIG_DIR` 指向的 profile `settings.json`，旧 provider 的 `ANTHROPIC_*` shell 变量即使残留也不会影响 claude 的行为。若后续恢复旧机制（直接 export `ANTHROPIC_*`），应使用 `Config.UsedVars()` 维护的全量变量清单做全量 `unset` 再重新 `export`。

> 旧机制下的例子：A 设了 `ANTHROPIC_BASE_URL=https://glm...`，切到官方 Claude 时若不 unset，残留的 `ANTHROPIC_BASE_URL` 会让 claude 仍走 GLM。新机制通过 `CLAUDE_CONFIG_DIR` 避免了这一问题。

---

## 2. 官方 Claude 作为"空 provider"

官方 Claude 使用 OAuth 登录态而非 API key，对应"什么都不 set"（`unset` 掉 `ANTHROPIC_BASE_URL` 等）。数据模型中 `env` 为空对象即代表此语义（见 [架构 §3](./architecture.md#3-数据模型元信息索引-profile-真值两层)）。

---

## 3. `current` 命令的语义正确性

`cc-select current` **必须读 shell 内的 `$CC_SELECT_ACTIVE` 环境变量**，而非磁盘文件。磁盘配置是全局共享的，读它会错误地报告"当前 shell 用的 provider"。这是 shell 级隔离正确性的关键。

> 即：磁盘上 providers 是"模板"，每个 shell 实际激活的是 `$CC_SELECT_ACTIVE`。`current` 反映的是后者。

---

## 4. 提示符可视化（满足 R6）

在 `PS1` 显示当前 provider（如 `[glm] %`）。多终端并行时用户极易忘记当前用的谁，可视化是体验加分项，近乎必需。

实现上需注意：不要破坏用户现有的 `PS1` 自定义，应提供可关闭/可自定义格式的开关。

---

## 5. API key 安全（满足 R7）

当前实现把 token 明文写入 `~/.cc-select/profiles/<id>/settings.json`（文件 0600、目录 0700），风险等级同 `~/.claude/settings.json`。README 需明确说明此风险。

**已预留的安全升级路径**：代码中已有 `internal/secrets/` 包和 keychain 占位机制（`$keychain:cc-select:<provider>:<var>`），未来可让 `add`/`edit`/Web 写入 keychain 占位而非明文，切换时由 `internal/secrets` 解析为真值后再写入 profile（或供 claude 读取）。

跨平台 keychain 方案：
- macOS：`security` / Keychain Services（通过 `zalando/go-keyring`）
- Linux：Secret Service API / libsecret（通过 `zalando/go-keyring`）
- Windows：Credential Manager（通过 `zalando/go-keyring`）

key 的存储与整体存储格式联动，见 [技术选型 - 存储格式选型](./tech-stack.md#4-存储格式json-两层元信息-profile-真值q3)。

---

## 6. claude 的 settings.json env 优先级高于 shell（已用 CLAUDE_CONFIG_DIR 解决）

> 本节记录 cc-select 机制层面最关键的发现与转折：旧的"shell 环境变量切换"被 claude 的 settings.json 覆盖而失效，最终改用 `CLAUDE_CONFIG_DIR` 解决。结论来自 2026-06-28 真机实测。

### 现象

`ccs use MiniMax` 后，shell 环境变量**确认切对**（`ANTHROPIC_BASE_URL=https://api.minimaxi.com/...`、`ANTHROPIC_MODEL=MiniMax-M2.7`），但 `claude` 启动后 `/status` 显示的仍是 settings.json 的 GLM 配置（`base URL=open.bigmodel.cn`、`model=glm-5.2`）。**cc-select 的 shell 切换对 claude 完全失效。**

### 根因：claude 优先用 settings.json 的 env，覆盖 shell 变量

claude 启动时，读取 `~/.claude/settings.json` 的 `env` 字段，并**以它为准覆盖 shell 环境变量**（至少对 `ANTHROPIC_*` 系列如此）。这与 Unix 常规"shell 环境变量优先"相反。

实测铁证（同一终端）：

| 时刻 | `ANTHROPIC_BASE_URL` | `ANTHROPIC_MODEL` |
|---|---|---|
| shell 里 `ccs use MiniMax` 后（未启动 claude） | `api.minimaxi.com`（MiniMax）✅ | `MiniMax-M2.7` ✅ |
| 启动 `claude` 后 `/status` | `open.bigmodel.cn`（settings.json 的 GLM）❌ | `glm-5.2` ❌ |

shell 是对的，是 claude 启动时用 settings.json 盖掉了它。

### 推翻此前假设

此前本文档（及讨论）假设"shell 环境变量优先于 settings.json，同名变量 shell 覆盖文件"。**实测证伪**：对 claude 而言，settings.json 的 env 优先级 ≥ shell。因此：

- ❌ "对齐变量名 → 同名覆盖"思路**无效**（同名时是 settings.json 覆盖 shell，不是反过来）。
- ❌ "切换时 unset 所有 ANTHROPIC_\*"思路**无效**（shell 里清空了，claude 启动仍从 settings.json 注入）。
- 只要 `~/.claude/settings.json` 有 `env` 字段，cc-select 的 shell 机制对 claude 就**无法生效**。

### 这意味着什么

cc-select 当前的"shell 环境变量隔离"机制：

| 机器状态 | cc-select 对 claude 是否有效 |
|---|---|
| settings.json **无** env（干净机器 / 未装 cc-switch） | ✅ 有效——claude 读 shell，cc-select 切换生效 |
| settings.json **有** env（装过 cc-switch / 手动配过） | ❌ **完全失效**——claude 读 settings.json，无视 shell |

cc-switch 选择"改 settings.json"正是这个原因——**只有 settings.json 才能真正影响 claude**。

### 解决方案：CLAUDE_CONFIG_DIR（方向 2，已采纳并实测验证 ✅）

shell 路线在"有 settings.json env"的机器上走不通，已改走 claude 配置层——**采用方向 2（多 CLAUDE_CONFIG_DIR）**，并已实测验证可行。

**机制**：每个 provider 一份独立配置目录 `~/.cc-select/profiles/<id>/settings.json`（含该 provider 的 env）。`ccs use X` 只 `export CLAUDE_CONFIG_DIR` 指向 X 的目录。claude 启动读该目录的 settings.json → 走 X 服务商。官方 provider 则 `unset CLAUDE_CONFIG_DIR`（回默认 `~/.claude`）。

**实测验证（2026-06-28）**：手动建 profile 目录 + `export CLAUDE_CONFIG_DIR` + `claude` → `/status` 确认走 MiniMax（base URL=minimaxi、model=MiniMax-M2.7），`unset` 后回默认。机制成立，且**不动 `~/.claude`**，与 cc-switch 完美共存。

**为何保留 shell 级隔离**：`CLAUDE_CONFIG_DIR` 本身是环境变量（per-shell），终端 A 指向 glm、终端 B 指向 deepseek，互不影响——cc-select 的初衷得以保留。每个 profile 的 settings.json 只放 env（`ANTHROPIC_AUTH_TOKEN` 在 env 里即完整认证，无需单独 credentials）。

详见 [架构 §2.0](./architecture.md#20-切换机制claude_config_dir关键)、profile 包（`internal/profile`）。

### 已知约束

- **token 明文**落 profile settings.json（claude 需读 env 值）。文件 0600、目录 0700。keychain 占位机制已预留（见本文 §5），尚未完全接入 CLI/Web 写入路径。
- **官方模式受用户 `~/.claude` 影响**：切官方 = 回默认目录，若用户默认 `~/.claude` 被 cc-switch 写过 env，官方模式下仍受其影响——本项目不接管 `~/.claude`，属用户环境。
- **旧 ANTHROPIC_\* 残留**：新机制不主动清 shell 旧变量，但 profile settings.json 的 env 会覆盖 claude 读取，无害。

