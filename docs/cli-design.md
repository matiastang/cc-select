# CLI 设计

> 本文聚焦命令行这一方向：`cc-select` 与 `ccs` 别名的关系、各子命令各自功能、`use` 为何需要 shell 函数。
> 上游：需求 [R2](./requirements.md#r2-命令行切换ccs)/[R3](./requirements.md#r3-cc-select-与-ccs-两个命令都要能用ccs-是-cc-select-的别名)，架构见 [架构设计](./architecture.md)。

---

## 1. 命令总览：`cc-select` 是主命令，`ccs` 是其短别名

用户有两个命令可用，**两者功能完全等价**：

| 命令 | 性质 | 说明 |
|---|---|---|
| `cc-select` | 主命令（CLI 二进制名） | 全功能，所有子命令都走它 |
| `ccs` | **别名**（短形式） | `cc-select` 的简写，省键入；二者可互换 |

> 设计意图（满足 [R3](./requirements.md#r3-cc-select-与-ccs-两个命令都要能用ccs-是-cc-select-的别名)）：`ccs` 不是"只管切换"的子集，而是 `cc-select` 的**完整别名**。`ccs use glm` 与 `cc-select use glm`、`ccs list` 与 `cc-select list` 完全等价。用户记不住全名时敲 `ccs`，写脚本/文档时用 `cc-select` 更清晰，两者皆可。

别名有两层实现（见本文 §4 的特殊性）：

- **普通子命令**（list/current/add/edit/remove/init/gui/language）：纯 alias，`ccs <sub>` 直接转发到 `cc-select <sub>` 即可。
- **切换命令 `use`**（特殊）：因需 `eval` 注入环境变量，`ccs` 是一个 **shell 函数**而非简单 alias，但对外仍表现为"和 cc-select use 等价"。

---

## 2. 子命令功能清单

| 子命令 | 功能 | 是否需要 eval | 典型用法 |
|---|---|---|---|
| `use <provider>` | **核心**：切换当前 shell 到指定 provider | ✅ 需要（shell 函数包装） | `ccs use glm` / `cc-select use glm` |
| `list` | 列出所有已配置 provider（标记当前 shell 激活项） | ❌ | `ccs list` |
| `current` | 显示**当前 shell** 激活的 provider（读 `$CC_SELECT_ACTIVE`，非磁盘） | ❌ | `ccs current` |
| `add <name>` | 交互式添加一个 provider，支持选择 preset 自动填充默认配置 | ❌ | `cc-select add glm --preset zhipu-glm` |
| `edit <name>` | 编辑指定 provider 的配置，支持 `--add-field` / `--remove-field` | ❌ | `cc-select edit glm --model glm-5` |
| `remove <name>` | 删除指定 provider | ❌ | `cc-select remove glm` |
| `mode [settings-only\|full]` | 查看或设置**全局**隔离模式 | ❌ | `cc-select mode settings-only` |
| `language [en\|zh]` | 查看或设置显示语言 | ❌ | `cc-select language zh` |
| `init` | 输出要追加到 `.zshrc`/`.bashrc`/`$PROFILE` 的 `ccs()` 函数代码 | ❌ | `cc-select init >> ~/.zshrc` |
| `gui` | 启动 GUI 配置界面（本地 Web 服务，见 [架构 §5](./architecture.md#5-gui-配置界面)） | ❌ | `cc-select gui` |

`add` / `edit` / `use` 均支持 `--mode settings-only|full|default` 以设置/覆盖隔离模式：
- `cc-select edit glm --mode full`：把 `glm` 的 per-provider 模式设为 Mode A（落盘到 `providers.json`）。
- `cc-select use glm --mode full`：**一次性**以 Mode A 切换，不落盘。

---

## 3. Provider Presets

`add` / `edit` 支持**内置 preset**（写死在二进制中的供应商模板），避免用户每次手写 `ANTHROPIC_BASE_URL`、模型映射等字段。

常见用法：

```bash
# 非交互式：直接指定 preset 并填 key
cc-select add ds --preset deepseek --api-key sk-xxx

# 覆盖默认模型
cc-select add ds --preset deepseek --api-key sk-xxx --model deepseek-chat

# 高级：覆盖 API 格式或认证字段
cc-select add ds --preset deepseek --api-key sk-xxx --api-format openai_chat --auth-field ANTHROPIC_API_KEY

# 覆盖任意 env 字段（如模型映射）
cc-select add ds --preset deepseek --api-key sk-xxx \
  --field ANTHROPIC_DEFAULT_SONNET_MODEL=claude-sonnet-5
```

交互式流程（省略 `--preset`）：

```bash
$ cc-select add glm
Available provider presets (choose a number or enter the preset id, leave empty for custom):

[Official]
  1. Claude Official
[China Official]
  2. 智谱 GLM
  3. DeepSeek
  4. Kimi
...
Preset: 2
ANTHROPIC_BASE_URL [https://open.bigmodel.cn/api/anthropic]:
ANTHROPIC_MODEL [glm-5.1]:
API key: sk-xxx
```

当前内置 preset 包括：

| Preset | 说明 |
|---|---|
| `claude-official` | Claude 官方，OAuth，无需 API key |
| `deepseek` | DeepSeek |
| `zhipu-glm` | 智谱 GLM（国内） |
| `zhipu-glm-en` | Zhipu GLM（国际版） |
| `kimi` | Moonshot Kimi |
| `kimi-coding` | Kimi for Coding |
| `openrouter` | OpenRouter |
| `siliconflow` | SiliconFlow |
| `volcano-agentplan` | 火山引擎 Agentplan |
| `aws-bedrock-aksk` | AWS Bedrock（AK/SK） |
| `aws-bedrock-apikey` | AWS Bedrock（API Key） |
| `github-copilot` | GitHub Copilot（OAuth） |
| `custom` | 空模板，完全手动填写 |

Preset 只决定**创建时的默认值**；保存后与普通 provider 一样可独立编辑、删除。

---

## 4. `use` 的特殊性：为何它需要 shell 函数而非 alias

`use` 是唯一需要改**当前 shell 环境**的命令，而子进程无法改父 shell 环境（见 [需求分析 §4](./requirements-analysis.md#4-核心架构约束动手前必读)）。因此 `use` 的二进制只**输出** shell 语句：

```bash
$ cc-select use glm
export CLAUDE_CONFIG_DIR='/Users/xxx/.cc-select/profiles/glm'
export CC_SELECT_ACTIVE='glm'
```

为免去手敲 `eval`，并让 `ccs use` 与 `cc-select use` 体验等价，`init` 注入的代码定义一个 `ccs()` shell 函数（由 `internal/rcinteg` 渲染为带 marker 块的启动脚本片段，见 [分发 §2](./distribution.md#2-web-配置页一键安装-shell-集成已实现)）。其逻辑等价于：

```bash
# ~/.zshrc（由 `cc-select init` 生成，实际输出为带 marker 的完整代码块）
ccs() {
  if [ "$1" = "use" ]; then
    eval "$(command cc-select use "${@:2}")"   # 切换：eval 注入环境
  else
    command cc-select "$@"                      # 其余子命令：直接转发
  fi
}
```

用户实际使用（`ccs` 与 `cc-select` 等价）：

```bash
ccs use glm         # 终端 A 切到 GLM（= cc-select use glm，经 eval 生效）
ccs use deepseek    # 终端 B 切到 DeepSeek（完全不影响终端 A）
ccs list            # 查看所有 provider（= cc-select list）
ccs current         # 查看当前 shell 用谁（= cc-select current）
claude              # 各终端用各自的 provider
```

### `use` 的内部流程

1. 从 `providers.json` 读取目标 provider；
2. 用 `prefs.ResolveMode` 解析最终隔离模式（一次性 `--mode` > provider 覆盖 > 全局 > 默认 Mode B）；
3. 调用 `profile.Sync(id, nil, mode)` 幂等构建/重建 profile（`env=nil` 表示沿用现有 profile 的 env）；
4. 输出由 `switcher.Plan` + `shell.Emit` 渲染的 export/unset 语句。

---

## 5. 交互式菜单（后续增强）

未来可为不带参数的 `ccs` 增加交互式选择菜单：

```
$ ccs
? Select provider for this shell:
> glm          (智谱 GLM)
  deepseek     (DeepSeek)
  official     (Claude 官方)  ← current
```

方向键选择、回车即切换（等价于 `ccs use <选中项>`）。这会降低用户记 provider 名字的负担，但当前版本尚未实现，属于路线图中体验增强项（见 [路线](./roadmap.md)）。
