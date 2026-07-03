# Windows 支持评估

> 本文评估 `cc-select` 在 Windows 上的可行性，回答需求 [Q6](./requirements.md#待用户确认的开放问题含已定决策)（macOS/Linux/Windows 三平台）。
> 结论先行：**可行 ✅**，Windows 上 shell 级隔离与 Unix 同构。本文给出机制对照、唯一限制、与 `cc-select` 各层的对接方式。

---

## 1. 结论

Windows 完全可行，无需降级需求。核心原因：PowerShell 的环境变量存在 **process scope**，语义与 Unix `export` 一致——只影响当前会话及其子进程，关闭即失，不污染全局。这正好是 `cc-select` 追求的 shell 级隔离。

**唯一限制**：Windows 上 **仅支持 PowerShell，不支持 CMD**。

| 平台 | 支持的 shell | 状态 |
|---|---|---|
| macOS / Linux | zsh（MVP）、bash/fish（扩展） | ✅ |
| Windows | **PowerShell**（5.1+ / 7） | ✅ |
| Windows | CMD（cmd.exe） | ❌ 不支持（见 §4） |

---

## 2. 机制对照：Unix vs Windows

| 概念 | macOS/Linux | Windows (PowerShell) | 等价? |
|---|---|---|---|
| 进程级环境变量（不持久） | `export VAR=value` | `$env:VAR = "value"` | ✅ 同义 |
| 清除环境变量 | `unset VAR` | `Remove-Item Env:\VAR` | ✅ |
| 持久化（**我们不用**） | — | `[Environment]::SetEnvironmentVariable("VAR","v","User")` 写注册表 | （对照用，禁用） |
| shell 启动脚本 | `~/.zshrc` | PowerShell `$PROFILE` | ✅ 对应 |
| 子进程继承环境变量 | ✅ | ✅ | ✅ |
| 切换函数注入 | `eval "$(cc-select use glm)"` | `Invoke-Expression (cc-select use glm)` 或点 sourcing | ✅ 等价 |

> **微软官方原文**（[about_Environment_Variables](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell/core/about/about_environment_variables)）：
> *"When you change environment variables in PowerShell, the change affects only the current session."*
>
> 即 `$env:` 设的是 **process scope**，不会出现在「系统属性 → 环境变量」里，关闭 PowerShell 即消失。这正是我们要的隔离语义。

---

## 3. 与 cc-select 各层对接

### 3.1 `use` 命令（切换）

Unix 二进制输出 `export CLAUDE_CONFIG_DIR=...`；Windows 输出等价 PowerShell 语句。CLI 按调用方 shell 类型（由 `init` 时记录或运行时检测）决定输出语法：

```
# Unix 输出
export CLAUDE_CONFIG_DIR='/Users/xxx/.cc-select/profiles/glm'
export CC_SELECT_ACTIVE='glm'

# Windows 输出（PowerShell）
$env:CLAUDE_CONFIG_DIR = '/Users/xxx/.cc-select/profiles/glm'
$env:CC_SELECT_ACTIVE = 'glm'
```

清理上一个 provider：Unix `unset CLAUDE_CONFIG_DIR`，Windows `Remove-Item Env:\CLAUDE_CONFIG_DIR`。

### 3.2 shell 集成（`init`）

`cc-select init` 按目标 shell 输出不同代码（[Q5](./requirements.md#待用户确认的开放问题含已定决策) 的可扩展设计在此落地）：

```powershell
# Windows: 写入 $PROFILE（PowerShell 的 .zshrc 等价物）
function ccs {
  if ($args[0] -eq 'use') {
    Invoke-Expression (cc-select use $args[1..($args.Length-1)])
  } else {
    cc-select @args
  }
}
```

`Invoke-Expression`（iex）是 PowerShell 的 `eval` 等价物——在当前会话执行 `cc-select use` 输出的 `$env:...` 语句，使变量注入本会话。

> **Web 一键安装的 `$PROFILE` 探测**（见 [distribution §2](./distribution.md#2-web-配置页一键安装-shell-集成已实现)）：PowerShell 的 `$PROFILE` 路径不硬算，而是委托 `pwsh -NoProfile -Command '$PROFILE'` 自报——它会返回 OneDrive 重定向后的真实绝对路径，零维护。`-NoProfile` 必加，避免启动时加载用户 profile（递归/慢/副作用）。Windows 上若没有 `pwsh` 再回退 `powershell.exe`；两者都没有则降级为手动指引。

### 3.3 Claude Code 兼容性

Claude Code 官方支持 **native Windows**（PowerShell/CMD 直接安装运行，无需 WSL/Admin），且 `ANTHROPIC_BASE_URL` 是官方支持的路由变量。因此在 PowerShell 里 `$env:ANTHROPIC_BASE_URL = "..."` 后启动 `claude`，与 Unix 行为一致。

### 3.4 Web GUI 与存储

- **Web GUI**：HTTP 服务 + 浏览器跨平台天然，零额外成本。
- **存储**：配置目录用 `%USERPROFILE%\.cc-select\`（等价 `~/.cc-select/`），JSON 格式跨平台一致。

---

## 4. 为何不支持 CMD

旧版 cmd.exe **没有函数和启动 profile 机制**，唯一的环境变量设置方式：

- `set VAR=value` —— 仅当前 cmd 进程内，但 cmd 无函数无法包装 `eval`，`ccs` 命令无从实现；
- `setx VAR value` —— 写注册表，**全局持久化、污染所有新进程**，正是 cc-select 要避免的"全局切换"。

因此 CMD 无法实现 shell 级隔离。**PowerShell 是 Windows 上的现代默认 shell**，Claude Code native Windows 也以 PowerShell 为主，故只支持 PowerShell 是合理且足够的限制。

---

## 5. 实现期的注意点

1. **`init` 须检测 shell 类型**：检测当前是 PowerShell 还是 zsh/bash，输出对应代码。PowerShell 下写到 `$PROFILE`，并提供路径提示。
2. **跨平台路径**：用语言的标准库处理 home 目录（不要硬编码 `~`），Windows 取 `USERPROFILE`。
3. **换行符**：PowerShell 语句用 `;` 分隔或换行；注意 CRLF。
4. **iex 安全**：`Invoke-Expression` 执行的是本工具自己生成的语句（非用户自由输入），安全可控；但仍应确保 `use` 输出做引号转义，避免 provider 名/值含特殊字符。
5. **Web 一键安装与 WSL**：Windows 原生 server（在 PowerShell/cmd 里跑 `cc-select gui`）访问不到 WSL 内的 `.bashrc`（`\\wsl$\...`）；WSL 用户需在 WSL 终端内自行 `cc-select init >> ~/.bashrc`。原生 PowerShell 走 `$PROFILE` 委托探测 + 三档降级，详见 [distribution §2](./distribution.md#2-web-配置页一键安装-shell-集成已实现)。

---

## 6. 验收

Windows 隔离用例并入 [acceptance-tests.md](./acceptance-tests.md) 的 AC1（隔离性）与 AC9（多 shell）：在 PowerShell 中两个窗口分别 `ccs use` 互不影响，行为与 zsh 对齐。

---

## 7. Smart App Control 与未签名可执行文件

> Windows 11 的 Smart App Control（SAC）会拦截**未签名 + 云信誉未知**的可执行文件。cc-select 是开源未签名项目，这同时影响 ① 开发者本机跑 dev build，② 用户下载 release。

### 机制

SAC（Win11 22H2+，全新安装时可选开启）对每个 exe 按 hash 查微软云信誉：签名可信 / 云已知良性 → 放行；已知恶意 → 拦；**未知（未签名 + 云里没有）→ 拦**（对陌生人零容忍）。SAC **只看 exe 本身，不看构建命令**——`make all` / `make dev` / `go build` 产出同一个未签名 exe，命运相同。SAC 开启后**关掉不可逆**（见下）。

### 对开发者本机的影响

开启 SAC 的机器上，`bin/cc-select.exe` 运行报「应用程序控制策略已阻止此文件」/ `Permission denied`，**dev build 无法本机跑**。绕过：

- **关 SAC**（永久不可逆，见下）——本机能跑，永久失 SAC 层（Windows Defender 仍保护）。
- **虚拟机**：Hyper-V / VMware 内的 Windows 不开 SAC，主机 SAC 不动。
- **CI**：`windows-latest` runner 默认不开 SAC，在 CI 实跑 Windows 集成测试（见 `.github/workflows/ci.yml` 的「Windows PowerShell integration」步骤）。

### 对发布（release）的影响

release exe 同样未签名。用户下载后：

- **未开 SAC 的用户（多数）**：首次运行触发 **SmartScreen** → 点「更多信息」→「仍要运行」。随下载量积累云信誉，警告减弱。
- **开了 SAC 的用户（少数）**：SAC 直接拦，**无法运行**，除非自行关 SAC。

缓解（按效果/成本）：代码签名（受信 CA，SAC + SmartScreen 都放行，证书 ~$200/年起；自签名两者都不认）> 信誉积累（仅减弱 SmartScreen，对 SAC 无效）> 文档说明（README 写清绕过步骤）。

### 本机关闭 SAC（仅测试用）

SAC 只能 UI 关（无命令行，防脚本绕过）：

1. 开始菜单 → **Windows 安全中心** → **应用和浏览器控制** → **Smart App Control 设置** → **关闭**。
2. 确认「关闭后无法重新打开」。

验证（PowerShell）：

```powershell
(Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\CI\Policy' -Name VerifiedAndReputablePolicyState).VerifiedAndReputablePolicyState
# 0=关闭 1=强制 2=评估
```

> ⚠️ **永久不可逆**：此机再也不能开 SAC（除非重装系统）。开发者机器的权衡是用 Windows Defender 替代 SAC 这一层。

---

## 来源

- [about_Environment_Variables — Microsoft Learn](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell/core/about/about_environment_variables)
- [Claude Code 环境变量文档](https://code.claude.com/docs/en/env-vars)（`ANTHROPIC_BASE_URL`）
- [Claude Code Advanced setup — Native Windows](https://code.claude.com/docs/en/setup)
- [Setting Windows PowerShell environment variables — Stack Overflow](https://stackoverflow.com/questions/714877/setting-windows-powershell-environment-variables)
