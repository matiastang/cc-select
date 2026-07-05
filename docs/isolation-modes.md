# 隔离粒度：全隔离 vs 仅 settings.json 隔离（双模式）

> 本文给出 cc-select 的「隔离粒度」设计与实现方案：在「整目录隔离」之外新增「仅隔离 `settings.json`、其余共享」的模式，支持全局默认 + 单 provider 覆盖，默认 Mode B。
> 背景见 [工程细节 §6](./engineering-decisions.md#6-为何改用-claude_config_dir)；数据模型见 [架构 §4](./architecture.md#4-数据模型元信息索引-profile-真值-偏好三层)。

---

## 1. 背景：问题出在哪

`ccs use X` 通过 `export CLAUDE_CONFIG_DIR` 把 claude 的配置家目录指向 `~/.cc-select/profiles/<id>/`。Claude Code 官方文档明确：`CLAUDE_CONFIG_DIR` 重定位的是**整个家目录**——

> All settings, **credentials, session history, and plugins** are stored under this directory.

而当前 `profile.Ensure()` 往该目录只写了一个 `{"env":{...}}`（见 `internal/profile/profile.go`）。结果：非官方 provider 激活时，这个 profile 目录里**除 settings.json 外什么都没有**，claude 从零开始建状态，导致两类本不该隔离的东西被隔离：

| 类别 | 被意外隔离的内容 | 后果 |
|---|---|---|
| **状态类** | `projects/`（对话历史）、`todos/`、`history.json`、`shell-snapshots/`、`statsig/` | 不同终端/provider 的历史、todo、命令历史互相看不到 |
| **配置类** | `plugins/`、`commands/`、`agents/`、`skills/`、`.mcp.json`；**以及用户 `~/.claude/settings.json` 里的非 env 字段**（permissions / hooks / model / theme…） | 全局插件/命令/agent/skill/MCP、全局权限与 hooks，在非官方 provider 下全部失效 |

> 关键事实（已查证）：Claude Code **没有**「只指向某个 settings.json」的环境变量；`CLAUDE_CONFIG_DIR` 是唯一杠杆，且它搬的是整个家。这决定了下方方案形态。

用户真实诉求其实只有一个：**在 shell 内隔离 provider 路由（settings.json 的 env），其余内容尽量共享。**

---

## 2. 两种模式

| | **Mode A：全隔离（= 改动前的现状）** | **Mode B：仅 settings.json 隔离（默认）** |
|---|---|---|
| profile 目录内容 | 只有真实 `settings.json` | 真实 `settings.json` ＋ 其余条目**链接**回 `~/.claude/`（Unix 软链；Windows 目录用 junction） |
| 对话历史 / todos / 命令历史 | 每 provider 独立 | **共享** |
| 插件 / 命令 / agent / skill / MCP | 看不到全局 | **共享** |
| 全局非 env 配置（permissions/hooks…） | 丢失 | **保留**（合并进 profile settings.json） |
| provider env 隔离（核心目标） | ✅ | ✅ |
| shell 级隔离（终端 A≠B） | ✅ | ✅（仍是 per-shell 环境变量） |
| settings.json 内容 | `{"env":...}`（或 web 原文） | `全局 settings.json` 的**整体 env 替换**为 provider env |
| 实现复杂度 | 低 | 中（链接维护 + settings 合并 + 平台分派） |

**心智模型（Mode B）**：profile 目录里「`settings.json` 是唯一的真实隔离文件，其余条目都是指向 `~/.claude/` 的链接」。settings.json 以外的所有读写透明落到共享的 `~/.claude`，自然共享；provider 路由仍由各自 profile 的 settings.json 决定，shell 级隔离不变。

### Mode B 目录示例（Unix）

```
~/.cc-select/profiles/glm/
├── settings.json              ← 真实文件（隔离）= 全局 settings.json 的 env 整体替换为 provider env
├── projects    -> ~/.claude/projects      ← 软链（共享）
├── todos       -> ~/.claude/todos
├── plugins     -> ~/.claude/plugins
├── commands    -> ~/.claude/commands
├── agents      -> ~/.claude/agents
├── skills      -> ~/.claude/skills
├── .mcp.json   -> ~/.claude/.mcp.json
├── history.json-> ~/.claude/history.json
└── …（~/.claude 下除 settings.json 外的每个条目，以及白名单预创建的条目）
```

---

## 3. 模式存储与切换（全局默认 + per-provider 覆盖）

### 3.1 两处存储

- **全局默认**：`~/.cc-select/prefs.json`（cc-select 自身偏好，区别于 claude 的 settings.json）：
  ```json
  { "isolationMode": "settings-only" }
  ```
- **per-provider 覆盖**：`providers.json` 里每个 `Provider` 增加可选字段 `isolationMode`（缺省 = 继承全局）：
  ```json
  { "providers": { "glm": { "id":"glm", "name":"智谱 GLM", "isolationMode":"full" } } }
  ```
  例：全局 Mode B，但 `glm` 单独覆盖为 Mode A（全隔离）。

### 3.2 优先级（高 → 低）

```
一次性 --mode（仅本次 use，不落盘）
   ↓ 未指定则
Provider.IsolationMode（providers.json 的 per-provider 覆盖）
   ↓ 未指定则
Prefs.IsolationMode（prefs.json 全局默认）
   ↓ 未指定则
Mode B（settings-only）  ← 兜底默认
```

### 3.3 切换方式

| 入口 | 作用 | 落盘位置 |
|---|---|---|
| `cc-select mode` | 打印全局模式 | — |
| `cc-select mode settings-only\|full` | 设置**全局**默认 | `prefs.json` |
| `cc-select edit <id> --mode settings-only\|full` | 设置某 provider 的覆盖；`--mode default` 清除覆盖（继承全局） | `providers.json` |
| `cc-select use <id> --mode full` | **一次性**，不改持久偏好 | 不落盘 |
| Web GUI | 全局开关 + 每 provider 选择器（含「继承全局」） | `prefs.json` / `providers.json` |

> 官方 provider（`claude-official`）不受模式影响：恒为 `unset CLAUDE_CONFIG_DIR`，回默认 `~/.claude`（天然全共享），无 profile 目录。

---

## 4. Mode B 机制详解

### 4.1 settings.json = 全局 settings 的 env 整体替换

Mode B 的 profile settings.json 是**派生产物**：

```
profile.settings.json = ( ~/.claude/settings.json 或 {} ) 然后 env 键整体替换为 providerEnv
```

规则（已定：**整体替换 env 块**）：

1. 读 `~/.claude/settings.json`（不存在则 `{}`）。
2. 解析为 `map[string]any`（**保留所有未知字段**——permissions/hooks/model/theme 等一个不丢）。
3. 把 `env` 键的值**整体替换**为 providerEnv（不做深合并；全局 settings 原有的 env 丢弃）。
4. 原子写回 profile settings.json（0600）。

边界：
- 全局 settings.json 解析失败 → 警告并降级为 `{"env":providerEnv}`（不阻断切换）。
- 序列化按字母序重排键（Go `json.Marshal` 对 map 行为），可接受的外观变化。

> **语义**：Mode B 下 profile settings.json 每次 `use` 重新生成，**不支持手改**。env 真值来源是 provider 配置；非 env 真值来源是 `~/.claude/settings.json`。Web 的「原文编辑 profile settings.json」是 Mode A 专属；Mode B 下 Web 改 provider env，非 env 引导用户改 `~/.claude/settings.json`。

### 4.2 链接共享 + 白名单预创建

把 `~/.claude/`（下称 `claudeHome`）的条目共享进 profile 目录：

- **denylist**：`settings.json`（恒真实、隔离）。
- **白名单 `sharedEntries`**（**单一维护点**）：需共享且需**预创建**的已知状态条目。claude 经 `$CLAUDE_CONFIG_DIR` 写新**目录**时目标必须存在，否则失败；故目录型需先在 `claudeHome` 建好再链接：
  ```
  sharedEntries = [
    {name:"projects",        dir:true},
    {name:"todos",           dir:true},
    {name:"shell-snapshots", dir:true},
    {name:"statsig",         dir:true},
    {name:"ide",             dir:true},
    {name:"plugins",         dir:true},
    {name:"commands",        dir:true},
    {name:"agents",          dir:true},
    {name:"skills",          dir:true},
    {name:"output-styles",   dir:true},
    {name:"history.json",    dir:false},
    {name:".mcp.json",       dir:false},
    {name:".claude.json",    dir:false, target:"../.claude.json"},  # home 根 sibling！
  ]
  ```
  > 该清单需随 claude 版本校准，是 Mode B 唯一的「跟随上游」维护点。
  > **`.claude.json` 特殊**：它在 home 根（`~/.claude.json`），不在 `~/.claude/` 内，故用 `target:"../.claude.json"` 把 profile 的 `.claude.json` 软链到 home 根那份共享（实测：`CLAUDE_CONFIG_DIR` 会重定位它，且 claude 启动必读，缺失即报错）。
- **共享算法（幂等、自愈）**，对 profile 目录执行：
  1. 白名单每个条目：dir 且 `claudeHome/<name>` 不存在 → 先 `MkdirAll`；再 `ensureLink`。
  2. `claudeHome` 中**实际存在**的其余条目（除 denylist）→ `ensureLink`（尽力共享 extras）。
  3. `ensureLink(profileDir, name, target, isDir)`：
     - `lstat profileDir/<name>`：
       - 不存在 → `makeLink`（平台分派，见 §4.4）。
       - 是链接且指向 target → 空操作。
       - 是链接但指向他处 → 删除重建（修陈旧）。
       - **真实条目（非链接）→ 权威化**：空的直接清掉重建；非空则**警告并跳过**（保持隔离，用户可手动 `rm` 后重新 `use`）。

> **文件型链接可悬挂**（Unix 软链 / Windows 见 §4.4）：`history.json` 即便 `claudeHome` 里还没有，链接也可先建——claude 写入穿透创建目标。**目录型必须预创建**，否则 `mkdir -p` 穿悬挂链接失败。

### 4.3 `use` 每次重建（已定）

正确性依赖「profile 始终反映当前全局 + provider env + 链接」。故 `ccs use X` 在输出 `export` **之前**先 `profile.Sync(id, env, mode)` 幂等重建：写合并后的 settings.json + 链接自愈。成本（一次 `readdir` + 小文件读写 + 若干 `lstat`）微秒到低毫秒级，可忽略；换来「改全局/装插件后下次 use 自动生效 + 链接断裂自愈」。eval 模式不变，`Sync` 只是二进制的副作用。

### 4.4 平台分派：链接怎么造

抽象一个 `makeLink(target, link string, isDir bool) error`，按平台实现：

| 平台 | 目录 | 文件 |
|---|---|---|
| **Unix**（macOS/Linux） | `os.Symlink`（免特权） | `os.Symlink` |
| **Windows** | **Junction**（`cmd /c mklink /J`，**免特权**） | `os.Symlink`（需开发者模式/管理员；**失败则跳过+警告**，属尽力而为） |

> Windows 用 junction 覆盖了 Mode B 要共享的**绝大多数（目录）**条目，且无需任何特权；仅 `history.json` 等个别文件在无开发者模式时不共享，影响很小。

---

## 5. 边界情况处理

### 5.1 从未用过 claude（无 `~/.claude`）或目录为空
- 没有条目可链接；全局 settings.json 不存在 → 合并退化为 `{"env":providerEnv}`。
- claude 首次启动 `CLAUDE_CONFIG_DIR=profile` 时在 profile 目录内 bootstrap 自己的状态。
- 此时 Mode B 行为等同 Mode A（无东西可共享），**正确无害**；日后产生 `~/.claude` 内容，下次 `use` 自愈即纳入共享。

### 5.2 有 `~/.claude` 但缺某些状态目录
- 白名单负责**预创建**缺失的共享目录，确保 claude 写入落到共享位置而非 profile 本地。

### 5.3 真实条目挡路（权威化，不做迁移）
- 项目尚未对外发布、本地 profile 数据可删，故**不建迁移工具**。
- `ensureLink` 遇真实（非链接）条目：**空的清掉重建**；**非空警告并跳过**（用户可 `rm -rf ~/.cc-select/profiles/<id>` 后重新 `use` 重建）。
- 即 Mode B 构造是权威的：profile 目录该是什么样就是什么样，不被陈旧实体阻挡（非空例外，避免静默毁数据）。

### 5.4 Windows
- 见 §4.4：目录用 junction（免特权）开箱即用；文件需开发者模式，失败降级为不共享该文件 + 警告。不强制 Mode A。

### 5.5 官方 provider
- 恒 `unset CLAUDE_CONFIG_DIR`，回 `~/.claude`，与模式无关；模式只作用于有 profile 目录的非官方 provider。

---

## 6. 已验证项

1. **`~/.claude.json`（home 下 sibling 大状态文件）是否被 `CLAUDE_CONFIG_DIR` 重定位？** **已验证 ✅**：实测确认 claude 会把 `.claude.json` 写进 `$CLAUDE_CONFIG_DIR/.claude.json`（每个 profile 一份），且它是 claude 启动必读的主配置（缺失会报 "Claude configuration file not found"）。**已处理**：`sharedEntries` 用 `targetRel:"../.claude.json"` 把 profile 的 `.claude.json` 软链到 home 根 `~/.claude.json`（sibling，不在 `~/.claude/` 内），统一共享 OAuth 账号/项目历史。
2. **`sharedEntries` 白名单与目标 claude 版本的实际状态目录一致**：作为 Mode B 唯一的"跟随上游"维护点，需在发版前根据目标 claude 版本复核。

> 早期还曾疑虑"设 `CLAUDE_CONFIG_DIR` 后 claude 是否完全不读全局 `~/.claude/settings.json`"；无论答案如何，Mode B 的合并逻辑都安全：若完全不看全局，则合并进 profile 的 permissions/hooks 等字段确保不丢失；若仍看全局，profile 中的 env 整体替换也保证 provider 路由正确。

---

## 7. 实现方案（组件化 / 模块化 / 可扩展）

### 7.1 新增与改动文件

```
internal/
├── prefs/                       # 【新包】cc-select 偏好 + 模式类型
│   ├── prefs.go                 #   Mode 类型/常量/默认值、Load/Save、prefs.json
│   ├── resolve.go               #   ResolveMode(override, provider, global) 三级优先级
│   └── prefs_test.go
├── profile/
│   ├── profile.go               # 【保留】Dir/Path/Remove/Exists/ReadEnv/ReadRaw
│   ├── build.go                 # 【新】Sync(id, env, mode) 统一构造器（编排）
│   ├── merge.go                 # 【新】mergeSettings(global, env) 整体替换 env
│   ├── link.go                  # 【新】ensureLink / shareEntries / 权威化（OS 中立逻辑）
│   ├── link_unix.go             # 【新,build !windows】makeLink = os.Symlink
│   ├── link_windows.go          # 【新,build windows】makeLink：dir→junction，file→Symlink(尽力)
│   ├── claudehome.go            # 【新】ClaudeHome() + sharedEntries 白名单
│   └── *_test.go
├── config/config.go             # 【改】Provider 增 IsolationMode 字段
├── app/app.go                   # 【改】App 增 Prefs，New() 一并 Load
├── cli/
│   ├── mode.go                  # 【新】cc-select mode [settings-only|full]
│   ├── use.go                   # 【改】--mode 一次性；输出前 Sync
│   ├── add.go / edit.go         # 【改】--mode per-provider；走 Sync
│   └── helpers.go               # 【改】writeProvider 透传 mode
└── web/api.go                   # 【改】模式 get/set；写 profile 走 Sync；Mode B 禁用原文编辑
```

### 7.2 关键类型与函数签名（草案）

```go
// internal/prefs/prefs.go
type Mode string
const (
    ModeSettingsOnly Mode = "settings-only" // Mode B（默认）
    ModeFull         Mode = "full"          // Mode A
)
const DefaultMode = ModeSettingsOnly

type Prefs struct { IsolationMode Mode `json:"isolationMode,omitempty"` }
func Load() (*Prefs, error)        // 缺文件 → IsolationMode="" （Resolve 时回退默认）
func Save(*Prefs) error            // 原子写 ~/.cc-select/prefs.json

// internal/prefs/resolve.go
// 三级优先级：一次性 > provider > 全局 > 默认。空串表示「未指定」，逐级回退。
func ResolveMode(oneOff, provider, global Mode) Mode

// internal/config/config.go （改动）
type Provider struct {
    ID            string            `json:"id"`
    Name          string            `json:"name"`
    Env           map[string]string `json:"env,omitempty"`
    IsolationMode prefs.Mode        `json:"isolationMode,omitempty"` // 【新】per-provider 覆盖；空=继承全局
}

// internal/profile/build.go
// add/edit/use/web 共用的唯一构造入口；幂等。Mode B 每次重建 settings.json 并自愈链接。
func Sync(id string, env map[string]string, mode prefs.Mode) (dir string, err error)

// internal/profile/merge.go —— 整体替换 env 块
func mergeSettings(globalJSON []byte, env map[string]string) ([]byte, error)

// internal/profile/link.go —— OS 中立
type skipEntry struct { Name string; Reason string }       // 非空真实条目跳过等
func ensureLink(profileDir, name, target string, isDir bool) (skipped bool, err error)
func shareEntries(profileDir, claudeHome string, deny []string) (skipped []skipEntry, err error)

// internal/profile/link_unix.go / link_windows.go —— 平台分派
func makeLink(target, link string, isDir bool) error

// internal/profile/claudehome.go
func ClaudeHome() (string, error)                          // 默认 ~/.claude
var sharedEntries = []entry{{"projects", true}, ...}       // 单一维护点
```

> **包依赖（无环）**：`prefs`（叶子）← `config`、`profile`；`config` 不导入 `profile`；`profile` 已导入 `config`。`Provider.IsolationMode` 用 `prefs.Mode` 类型 → `config` 导入 `prefs`（单向，无环）。

### 7.3 数据流（统一写路径 + 三级模式解析）

```
use  : mode = ResolveMode(--mode 一次性, provider.IsolationMode, prefs.IsolationMode)
add/edit/web : mode = ResolveMode(无, provider.IsolationMode(--mode 可设), prefs.IsolationMode)
   └─ profile.Sync(id, env, mode)
        ├─ Mode A：写 {"env":env}（= 改动前 profile.Ensure）；不碰链接
        └─ Mode B：
            ├─ mergeSettings(读 ~/.claude/settings.json, env) → 写 profile settings.json
            └─ shareEntries：白名单预创建 + ensureLink 自愈 + 收集 skipped(非空真实条目)
```

- `use`：`Sync` 后照旧 `switcher.Plan` → `shell.Emit` → stdout（供 eval）。**eval 模式不变**。
- `web`（Mode B）：编辑 provider env → `Sync`；隐藏「原文编辑 profile settings.json」（仅 Mode A）。

### 7.4 关键算法（伪码）

```text
Sync(id, env, mode):
  if id == official: return "", nil                 # 官方无 profile
  dir = profile.Dir(id); MkdirAll(dir, 0700)
  switch mode:
   case Full:
      write {env} → dir/settings.json               # 改动前行为，零风险
   case SettingsOnly:
      g = read(ClaudeHome/settings.json)            # 缺则 {}
      merged = mergeSettings(g, env)                # 整体替换 env
      write merged → dir/settings.json              # 原子写 0600
      skipped = shareEntries(dir, ClaudeHome, deny=[settings.json])
      if skipped: warn(stderr, 列出未链接项及原因)
  return dir

ensureLink(dir, name, target, isDir):
  p = dir/name; li = lstat(p)
  if not exist: return makeLink(target, p, isDir)
  if isLink(li):
     if resolvesTo(li)==target: return              # 正确，空操作
     else: remove(p); return makeLink(target, p, isDir)   # 修陈旧
  # 真实条目 → 权威化
  if isEmpty(p): remove(p); return makeLink(target, p, isDir)   # 空的清掉重建
  else: return skip(name, "非空真实条目，已跳过")              # 非空警告跳过

mergeSettings(global, env):                          # 整体替换 env 块
  m = unmarshal<map[string]any>(global or {})        # 保留所有未知字段
  m["env"] = env                                      # 整体替换（非深合并）
  return marshal(m)

ResolveMode(oneOff, provider, global):
  if oneOff   != "": return oneOff
  if provider != "": return provider
  if global   != "": return global
  return DefaultMode                                  # settings-only
```

### 7.5 测试策略

- `prefs`：缺文件→空值（Resolve 回退默认）；存取往返；坏 JSON 报错；`ResolveMode` 三级回退组合。
- `merge`：保留未知字段；env 整体替换；全局非法 JSON 降级；空全局。
- `link`：`ensureLink` 的六分支（不存在/正确链接/陈旧链接/空真实/非空真实跳过/目录预创建）；悬挂文件链接可写。
- `build/Sync`：Mode A 等价旧 `Ensure`；Mode B 产物正确；无 `~/.claude` 不报错；幂等；skipped 上报。
- 平台：`makeLink` 在 Unix 造符号链接；Windows 造 junction（集成测试，CI windows job）。
- `use`：Mode B 下先写后输出、eval 语句不变（沿用 `switcher_test` 思路）；`--mode` 一次性覆盖。

---

## 8. 扩展点（设计已为其留口）

| 未来需求 | 扩展方式 |
|---|---|
| 新增 claude 状态目录共享 | 加到 `sharedEntries` 一个条目（§4.2 单一维护点） |
| 新增隔离模式（如「仅 settings.json 隔离 + 历史也隔离」） | `prefs.Mode` 加常量 + `Sync` 加分支 |
| 新增偏好（PS1 格式、默认 shell…） | `Prefs` 加字段（具名结构体，缺省即默认） |
| per-provider 覆盖 | **v1 已实现**（`Provider.IsolationMode` + 三级 `ResolveMode`） |
| env 深合并 | 当前整体替换；日后改 `mergeSettings` 合并逻辑即可，调用方不变 |
| keychain 真值接入 | `Sync` 收到的 env 已是真值即可；合并/链接逻辑无关 |
| Windows 文件级共享（免特权） | `link_windows.go` 的文件分支由「尽力 symlink」改硬链接 |

---

## 9. 已定决策

| # | 决策 |
|---|---|
| 1 | 默认 **Mode B**（`settings-only`） |
| 2 | 全局默认 Mode B + **真正支持 per-provider 覆盖**；优先级：一次性 `--mode` > provider > 全局 > 默认 |
| 3 | Windows 用 **junction 共享目录（免特权）**；文件尽力而为（无开发者模式则跳过+警告） |
| 4 | **不做迁移**；Mode B 构造权威化（空真实条目清掉重建，非空警告+跳过） |
| 5 | profile settings.json 的 env **整体替换**为 provider env（非深合并） |
| 6 | `use` **每次重建** profile（幂等，自愈） |
