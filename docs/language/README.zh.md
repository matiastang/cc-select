# cc-select

**[English](../../README.md) | [中文](./README.zh.md) | [日本語](./README.ja.md)**

Claude Code 的 Shell 级 AI 服务商隔离 —— 每个终端窗口可自选 provider。

`cc-select` 让同一台机器上的不同终端窗口在使用 Claude Code 时选择不同的 AI 模型服务商。它是 [cc-switch](https://github.com/farion1231/cc-switch) 的 shell 级对应方案：cc-switch 通过改写 `~/.claude/settings.json` 做全局切换，而 `cc-select` 只影响当前终端及其子进程。

## 安装

### macOS / Linux（Homebrew）

一条命令安装（无需先执行 `brew tap`）：

```bash
brew install matiastang/cc-select/cc-select
```

也可以先显式添加 tap 再安装：

```bash
brew tap matiastang/cc-select
brew install cc-select
```

### Windows（Scoop）

一条命令安装（无需先执行 `scoop bucket add`）：

```powershell
scoop install https://raw.githubusercontent.com/matiastang/scoop-cc-select/main/cc-select.json
```

也可以先显式添加 bucket 再安装：

```powershell
scoop bucket add cc-select https://github.com/matiastang/scoop-cc-select
scoop install cc-select
```

### macOS / Linux（安装脚本）

如果你没有 Homebrew，可以用官方脚本安装或更新：

```bash
curl -fsSL https://raw.githubusercontent.com/matiastang/cc-select/main/scripts/install.sh | sh
```

安装到指定目录：

```bash
curl -fsSL https://raw.githubusercontent.com/matiastang/cc-select/main/scripts/install.sh | sh -s -- --dir /usr/local/bin
```

脚本会自动检测已有安装的目录并替换其中的二进制；否则默认安装到 `~/.local/bin`（必要时使用 `/usr/local/bin`）。

### Windows（安装脚本）

如果你没有 Scoop，可以用官方 PowerShell 脚本安装或更新：

```powershell
irm https://raw.githubusercontent.com/matiastang/cc-select/main/scripts/install.ps1 | iex
```

脚本会安装到 `%LOCALAPPDATA%\cc-select`，将其加入用户 PATH，并原地更新已有安装。

> 注意：Windows ARM64 尚未支持，目前仅发布 Windows amd64 构建。

### 手动安装

从 [GitHub Releases](https://github.com/matiastang/cc-select/releases) 下载对应平台的压缩包，将 `cc-select`（Windows 为 `cc-select.exe`）解压到 `PATH` 中的某个目录，然后继续下面的 shell 集成步骤。

## 快速开始

### 1. Shell 集成

`cc-select init` 会输出 `ccs` 所需的 shell 包装函数。把它追加到 shell 启动文件后重载即可：

```bash
# macOS / Linux — zsh
cc-select init >> ~/.zshrc && source ~/.zshrc

# macOS / Linux — bash
cc-select init >> ~/.bashrc && source ~/.bashrc
```

```powershell
# Windows — PowerShell
cc-select init >> $PROFILE
```

> 已支持的 shell：**zsh / bash / PowerShell**。Windows 的 CMD 不支持；fish 暂未支持。

### 2. 添加服务商

推荐通过 Web 配置界面操作：

```bash
cc-select gui
```

也可以在命令行中添加：

```bash
cc-select add glm
```

### 3. 查看已添加的服务商

```bash
ccs list
```

### 4. 只在当前 shell 切换到该服务商

```bash
ccs use glm
```

## Windows 首次运行（SmartScreen / Smart App Control）

cc-select 是**未签名的开源**二进制。在 Windows 上：

- **SmartScreen**（所有用户）：首次运行可能提示「Windows 已保护你的电脑」——点「更多信息」→「仍要运行」。
- **Smart App Control (SAC)**（仅当你开启时）：SAC 会拦截未签名/未知 exe，且**没有**「仍要运行」选项。若已开启，需关闭 SAC（永久不可逆）或在未开启 SAC 的机器上运行。详见 [docs/windows-support.md §7](../windows-support.md#7-smart-app-control-与未签名可执行文件)。

## 工作原理

子进程无法修改父 shell 的环境变量，因此 `cc-select` 拆成两层协作：

1. `cc-select` 二进制只**打印** shell 语句（主要是 `export CLAUDE_CONFIG_DIR=...`）。
2. `ccs()` shell 函数（由 `cc-select init` 注入到 `~/.zshrc` 等启动脚本）在当前 shell 中 `eval` 这些语句。

`cc-select use <provider>` 会导出 `CLAUDE_CONFIG_DIR`，指向独立的 profile 目录（`~/.cc-select/profiles/<provider>/settings.json`）。Claude Code 启动时读取该目录的 env，从而实现“每个终端各自的服务商”。

## 隔离模式

- **Mode B — `settings-only`（默认）**：每个 provider 仅隔离 `settings.json`；历史、插件、commands 等通过链接共享回 `~/.claude`。
- **Mode A — `full`**：整个 profile 目录完全隔离。

使用 `cc-select mode` 查看/设置全局默认；用 `cc-select edit <id> --mode ...` 或 `ccs use <id> --mode ...` 做 per-provider 覆盖或一次性覆盖。详见 [docs/isolation-modes.md](../isolation-modes.md)。

## 安全说明

API key 目前以**明文**存储在 `~/.cc-select/profiles/<id>/settings.json` 中（文件权限 `0600`，目录权限 `0700`）。风险等级与 `~/.claude/settings.json` 相同。后续计划接入系统 Keychain；keychain 占位机制与 `internal/secrets` 包已实现，待接入 CLI/Web 写入路径。

## 构建

```bash
make all      # 构建前端 + 二进制，输出到 ./bin/cc-select
make test     # 运行 Go 单元测试
make vet      # 运行 go vet
make e2e      # 运行 Playwright 端到端测试
make check    # 运行所有静态检查（格式、类型、Lint、脚本、mod tidy）
```

## 本地开发

安装依赖并注册 git hooks（在 `internal/frontend` 执行 `npm install` 时会自动安装 hooks）：

```bash
cd internal/frontend && npm install
```

本地运行所有静态检查：

```bash
make check
```

自动格式化全部代码：

```bash
make fmt
```

`git commit` 时会通过 pre-commit hook 自动校验；任何静态检查不通过都会拦截提交。

## 许可

Apache License 2.0
