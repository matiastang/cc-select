# CLI 设计

> 本文聚焦命令行这一方向：`cc-select` 与 `ccs` 别名的关系、8 个子命令各自功能、`use` 为何需要 shell 函数。
> 上游：需求 [R2](./requirements.md#r2-命令行切换ccs)/[R3](./requirements.md#r3-cc-select-与-ccs-两个命令都要能用ccs-是-cc-select-的别名)，架构见 [架构设计](./architecture.md)。

---

## 1. 命令总览：`cc-select` 是主命令，`ccs` 是其短别名

用户有两个命令可用，**两者功能完全等价**：

| 命令 | 性质 | 说明 |
|---|---|---|
| `cc-select` | 主命令（CLI 二进制名） | 全功能，所有子命令都走它 |
| `ccs` | **别名**（短形式） | `cc-select` 的简写，省键入；二者可互换 |

> 设计意图（满足 [R3](./requirements.md#r3-cc-select-与-ccs-两个命令都要能用ccs-是-cc-select-的别名)）：`ccs` 不是"只管切换"的子集，而是 `cc-select` 的**完整别名**。`ccs use glm` 与 `cc-select use glm`、`ccs list` 与 `cc-select list` 完全等价。用户记不住全名时敲 `ccs`，写脚本/文档时用 `cc-select` 更清晰，两者皆可。

别名有两层实现（见本文 §3 的特殊性）：

- **普通子命令**（list/current/add/edit/remove/init/gui）：纯 alias，`ccs <sub>` 直接转发到 `cc-select <sub>` 即可。
- **切换命令 `use`**（特殊）：因需 `eval` 注入环境变量，`ccs` 是一个 **shell 函数**而非简单 alias，但对外仍表现为"和 cc-select use 等价"。

---

## 2. 子命令功能清单

| 子命令 | 功能 | 是否需要 eval | 典型用法 |
|---|---|---|---|
| `use <provider>` | **核心**：切换当前 shell 到指定 provider | ✅ 需要（shell 函数包装） | `ccs use glm` / `cc-select use glm` |
| `list` | 列出所有已配置 provider（标记当前 shell 激活项） | ❌ | `ccs list` |
| `current` | 显示**当前 shell** 激活的 provider（读 `$CC_SELECT_ACTIVE`，非磁盘） | ❌ | `ccs current` |
| `add <name>` | 交互式添加一个 provider | ❌ | `cc-select add glm` |
| `edit <name>` | 编辑指定 provider 的配置 | ❌ | `cc-select edit glm` |
| `remove <name>` | 删除指定 provider | ❌ | `cc-select remove glm` |
| `init` | 输出要追加到 `.zshrc`/`.bashrc` 的别名 + shell 函数代码 | ❌ | `cc-select init >> ~/.zshrc` |
| `gui` | 启动 GUI 配置界面（桌面 App 或本地 Web 服务，见 [架构 §4](./architecture.md#4-gui-配置界面)） | ❌ | `cc-select gui` |

> 注：不带参数的 `ccs`（即 `cc-select`）可设计为交互式 provider 选择菜单（方向键选、回车切换），作为 `use` 的便捷入口。

---

## 3. `use` 的特殊性：为何它需要 shell 函数而非 alias

`use` 是唯一需要改**当前 shell 环境**的命令，而子进程无法改父 shell 环境（见 [需求分析 §3](./requirements-analysis.md#3-核心架构约束动手前必读)）。因此 `use` 的二进制只**输出** shell 语句：

```bash
$ cc-select use glm
export CLAUDE_CONFIG_DIR='/Users/xxx/.cc-select/profiles/glm'
export CC_SELECT_ACTIVE='glm'
```

为免去手敲 `eval`，并让 `ccs use` 与 `cc-select use` 体验等价，`init` 注入的代码同时定义**别名**与**切换函数**：

```bash
# ~/.zshrc（由 `cc-select init` 生成）
alias cc-select-use='cc-select use'   # 直跑二进制，会打印 export 语句（不自动生效）

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

---

## 4. 交互式菜单（可选增强）

不带参数的 `ccs` 弹出交互式选择菜单：

```
$ ccs
? Select provider for this shell:
> glm          (智谱 GLM)
  deepseek     (DeepSeek)
  official     (Claude 官方)  ← current
```

方向键选择、回车即切换（等价于 `ccs use <选中项>`）。这降低了用户记 provider 名字的负担，是 MVP 之后的体验增强（见 [路线](./roadmap.md)）。
