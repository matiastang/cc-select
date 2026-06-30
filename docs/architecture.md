# 架构设计

> 本文给出 `cc-select` 的整体架构：双形态分工、eval 两层结构、数据模型、配置生效语义。
> 上游：需求 [R1](./requirements.md#r1-shell-级模型服务商隔离)/[R4](./requirements.md#r4-图形化界面配置服务商)，约束见 [需求分析](./requirements-analysis.md#3-核心架构约束动手前必读)。

---

## 1. 双形态：GUI 配置 + CLI 切换

`cc-select` 是**双形态**工具：GUI 负责"配置 provider"，CLI + `ccs` 命令负责"切换 provider"。两者读写**同一份配置**，职责分离、天然解耦。

```
┌─────────────────────────────────────────────────────────┐
│  GUI 配置界面  (provider 的增删改查,可视化表单)            │
│   - 列出所有 provider,点击编辑(URL/Key/Model 输入框)      │
│   - 形态待定: 桌面 App / 本地 Web 服务(见本文 §4)         │
└───────────────────────────┬─────────────────────────────┘
                            │ 读写
                            ▼
        ┌───────────────────────────────────────┐
        │  共享配置存储 (待定:JSON/SQLite/多文件)  │
        │  ← GUI 写配置,CLI 读配置                │
        └───────────────────┬───────────────────┘
                            │ 读取
                            ▼
┌─────────────────────────────────────────────────────────┐
│  cc-select CLI 二进制  (读配置,输出 export 语句)           │
│   - use / list / current / init                          │
│   - 只"输出"要 export 的内容,绝不改调用方 shell 环境       │
└───────────────────────────┬─────────────────────────────┘
                            │  cc-select use glm → 输出 export 语句
                            ▼
┌─────────────────────────────────────────────────────────┐
│  ccs 别名 + shell 函数  (装到 ~/.zshrc / ~/.bashrc)        │
│   - ccs 是 cc-select 的短别名,二者功能等价                │
│   - 仅 use 走 eval "$(cc-select use glm)" ← 关键这步       │
│   - 由 shell 自己执行 export,因此能改"当前"shell 环境       │
│   - 维护 CC_SELECT_ACTIVE 变量;在 PS1 显示当前 provider   │
└───────────────────────────┬─────────────────────────────┘
                            │
                            ▼
            claude  (继承父 shell 的环境变量,用对应服务商)
```

职责划分：
- **GUI** → 配置（慢工出细活：填长 key、批量管理、可视化）。
- **`ccs` 命令** → 切换（秒切：终端敲一行）。

---

## 2. 切换层：CLI + shell wrapper（核心约束）

切换这条链路必须拆成 CLI 二进制 + `ccs` shell 函数两层，原因见 [需求分析 §3](./requirements-analysis.md#3-核心架构约束动手前必读)（子进程不能改父 shell 环境）。

`ccs` 不是二进制，而是 shell 函数——只有函数体内的 `eval` 才能在"当前这个 shell"里执行 export。命令细节见 [CLI 设计](./cli-design.md)。

### 2.0 切换机制：CLAUDE_CONFIG_DIR（关键）

**`ccs use X` 不再 export 一堆 `ANTHROPIC_*`，而是只 export 一个 `CLAUDE_CONFIG_DIR` 指向 X 的独立配置目录。**

背景：实测确认 claude 启动时优先用 `~/.claude/settings.json` 的 env、**覆盖** shell 环境变量（见 [工程细节 §6](./engineering-decisions.md#6-claude-的-settingsjson-env-优先级高于-shell已用-claude_config_dir-解决)）。所以 export `ANTHROPIC_*` 对 claude 无效。改用 claude 官方支持的 `CLAUDE_CONFIG_DIR`——指向独立目录后，claude 读该目录的 `settings.json`，隔离且生效。

- **普通 provider**：`export CLAUDE_CONFIG_DIR=~/.cc-select/profiles/<id>`（该目录的 `settings.json` 含 X 的 env）
- **官方 provider**（`claude-official`）：`unset CLAUDE_CONFIG_DIR`，让 claude 回默认 `~/.claude`（复用用户既有登录态/全局配置，与 cc-switch 完美共存）

shell 集成机制（`eval` 注入）不变，只是注入的变量从"一堆 ANTHROPIC_*"换成"一个 `CLAUDE_CONFIG_DIR`"。

> **隔离粒度（双模式）**：`CLAUDE_CONFIG_DIR` 重定位的是整个家目录，默认会把对话历史/插件/全局非 env 配置也一并隔离。cc-select 提供两种粒度，默认 **Mode B（仅 settings.json 隔离）**：profile 目录除真实 `settings.json` 外，其余条目软链回 `~/.claude` 共享；深度用户可切回 **Mode A（全隔离，= 现状）**。机制、边界与切换见 [隔离粒度设计](./isolation-modes.md) 与 [工程细节 §7](./engineering-decisions.md#7-隔离粒度全隔离-vs-仅-settingsjson-隔离双模式)。

### 2.1 跨平台约束（满足 Q6：macOS/Linux/Windows）

需同时跑在三大平台，架构各层需注意：

| 层 | 跨平台要点 |
|---|---|
| 环境变量隔离机制 | macOS/Linux 用 `export`/`unset`；Windows PowerShell 用 `$env:VAR`（process scope，**与 `export` 同义**）。详见 [Windows 评估](./windows-support.md)。 |
| shell 集成 | MVP 先做 **zsh**（[Q5](./requirements.md#待用户确认的开放问题含已定决策)），`init` 输出按 shell 类型分发（zsh/bash/fish/PowerShell）。 |
| 存储位置 | 配置目录按 OS 惯例（`~/.cc-select/` 可统一，或 Windows 用 `%USERPROFILE%`）。 |
| Web GUI | HTTP 服务 + 浏览器跨平台天然一致，无额外成本。 |

> **Windows 评估结论：可行 ✅**。经评估，Windows 上 shell 级隔离与 Unix 同构——PowerShell 的 `$env:VAR`（process scope，不持久）等价于 `export`，子进程同样继承；Claude Code 官方支持 native Windows + `ANTHROPIC_BASE_URL` 路由。唯一限制：**仅支持 PowerShell，不支持 CMD**（CMD 无函数/profile 机制，只有全局污染的 `setx`）。详见 [Windows 支持评估](./windows-support.md)。

---

## 3. 数据模型（元信息索引 + profile 真值 + 偏好，三层）

配置分三层存储（都 JSON，原子写；providers.json/profile 文件 0600、prefs.json 0600）：

**① providers.json（元信息索引）** `~/.cc-select/providers.json`：只存 id/name，**不含 env、不含 token**。

```json
{
  "providers": {
    "glm": { "id": "glm", "name": "智谱 GLM" },
    "claude-official": { "id": "claude-official", "name": "Claude 官方" },
    "deepseek": { "id": "deepseek", "name": "DeepSeek" }
  }
}
```

**② profile settings.json（env 真值，claude 读这个）** `~/.cc-select/profiles/<id>/settings.json`：含该 provider 的 env（可能含敏感 token）。官方 provider 无 profile（切它 = unset `CLAUDE_CONFIG_DIR`）。**Mode B（默认）** 下，该目录还会用链接（Unix 软链 / Windows junction）共享 `~/.claude` 的其余条目（历史/插件/命令/agent/skill/MCP…），且 settings.json = 全局 `~/.claude/settings.json` 的 **env 整体替换**为 provider env；**Mode A** 下目录只有 settings.json。详见 [隔离粒度设计](./isolation-modes.md)。

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "https://open.bigmodel.cn/api/anthropic",
    "ANTHROPIC_AUTH_TOKEN": "your-token",
    "ANTHROPIC_MODEL": "glm-5.2"
  }
}
```

**③ prefs.json（cc-select 自身偏好）** `~/.cc-select/prefs.json`：存全局偏好，目前仅全局 `isolationMode`（`settings-only`=Mode B 默认 ｜ `full`=Mode A）。**per-provider 覆盖**存于 `providers.json` 的 `Provider.IsolationMode`（缺省=继承全局）；优先级：一次性 `--mode` > provider > 全局 > 默认(Mode B)。缺文件即默认值。

> 关系：providers.json 是权威目录（谁存在、叫什么）；profile 目录是 claude 实际读的运行时配置；prefs.json 是全局偏好（隔离模式等，per-provider 覆盖则记在 providers.json）。add/edit/use/web 通过统一的 `profile.Sync(id, env, mode)` 构造 profile 目录——mode 由 `prefs.ResolveMode(一次性, provider, 全局)` 三级解析（Mode B 合并 settings + 链接共享，Mode A 仅写 env）——再写 providers.json；remove 先删 profile 目录、再删 providers.json 条目。
>
> **API key 存储方式**：当前实现把 token 明文写入 profile settings.json（文件 0600、目录 0700）。代码中已保留 keychain 占位机制（`$keychain:cc-select:<provider>:<var>`）与 `internal/secrets` 抽象，可作为后续升级安全存储的路径。详见 [技术选型 - 存储格式](./tech-stack.md#4-存储格式json-两层元信息-profile-真值q3)。

---

## 4. GUI 配置界面

GUI 专门做 provider 的可视化配置（增删改查），借鉴并尽量对齐 cc-switch 的体验。

**当前选定形态：本地 Web 服务 + 浏览器**（满足 [R4](./requirements.md#r4-图形化界面配置服务商)/[Q2](./requirements.md#待用户确认的开放问题含已定决策)）。`cc-select gui` 起一个本地 HTTP 服务，自动打开浏览器访问配置页；该页通过 HTTP API 读写共享配置存储。

| 方案 | 状态 | 说明 | 优点 | 缺点 |
|---|---|---|---|---|
| **本地 Web 服务 + 浏览器** | ✅ **选定** | `cc-select gui` 起服务，自动开浏览器 | 无需打包桌面 App、跨平台天然、与 CLI 语言解耦 | 非常驻，每次要起服务 |
| 桌面 App（Tauri/Electron/Wails） | 🔁 备选 | 独立窗口 / 系统托盘（对标 cc-switch） | 体验最好，可常驻 | 需打包，体积较大 |

> 选定 Web 路线的关键收益：**GUI 与 CLI 实现语言解耦**——任何语言都能起 HTTP 服务 + 读写 JSON，因此语言选型（[Q1](./requirements.md#待用户确认的开放问题含已定决策)）可独立按 CLI 开发效率决定，不被 GUI 绑架。日后若要切换为桌面 App，因配置存储已是共享文件（见 §3），GUI 形态可平滑替换，对 CLI 零影响。

---

## 5. 配置生效语义（满足 R8，必须讲清）

引入 GUI 后，配置和切换分属两个进程，会带来一个和纯命令行不同的心智模型问题：

> 在 GUI 里改了某个 provider 的配置后，**已经在运行、或已经 `ccs` 选过的终端，会自动跟着变吗？**

**答：不会，也不应该会。** 这是 shell 级隔离的必然结果。

- GUI 改的是**磁盘上**的 profile settings.json；
- 终端里 `ccs use glm` 当时已 `export CLAUDE_CONFIG_DIR` 指向该 profile 目录。新机制下，下次该终端启动 claude 时会读 profile 目录的 settings.json——所以**若 GUI 改了同一个 profile，新启动的 claude 会吃到新值**（无需重新 `ccs use`）；但**已在运行的 claude 进程**不会变（进程已加载配置）。

| 你改/做的 | 影响谁 | 何时生效 |
|---|---|---|
| 在 GUI 里改 provider 配置 | 改的是 profile settings.json | 该 profile 对应终端**下次启动 claude** 时生效 |
| 在终端里 `ccs use glm` | 只影响**这个终端** | 立即（CLAUDE_CONFIG_DIR 指向 glm profile） |

所以心智模型：**换服务商要 `ccs use <name>`；改某服务商配置后，重启该终端的 claude 即可吃到新配置。**

这与 cc-switch 不同（cc-switch 改全局文件，所有终端重启 claude 都吃新配置），但这是 `cc-select` 追求"shell 级隔离"的代价，是 feature 不是 bug。可选的体验优化：GUI 改完 provider 后提示"已在使用的终端需重新 `ccs use <name>` 生效"——但**不自动同步**，自动同步反而会破坏隔离语义。

验收用例见 [验收测试](./acceptance-tests.md)。
