# cc-select

**[English](../../README.md) | [中文](./README.zh.md) | [日本語](./README.ja.md)**

Shell 级 AI 服务商隔离 —— 每个终端窗口自选其 provider。

`cc-select` 让同一台机器上的不同终端窗口在使用 Claude Code 时选择不同的 AI 模型服务商。它是 [cc-switch](https://github.com/farion1231/cc-switch) 的 shell 级对应方案：cc-switch 通过改写 `~/.claude/settings.json` 做全局切换，而 `cc-select` 只影响当前终端及其子进程。

## 快速开始

```bash
# 安装二进制并加入 PATH，然后注入 shell 包装函数
cc-select init >> ~/.zshrc
source ~/.zshrc

# 添加一个服务商
cc-select add glm

# 只在当前 shell 切换到该服务商
ccs use glm

# 打开 Web 配置界面
cc-select gui
```

## 工作原理

子进程无法修改父 shell 的环境变量，因此 `cc-select` 拆成两层协作：

1. `cc-select` 二进制只**打印** shell 语句（主要是 `export CLAUDE_CONFIG_DIR=...`）。
2. `ccs()` shell 函数（由 `cc-select init` 注入到 `~/.zshrc` 等启动脚本）在当前 shell 中 `eval` 这些语句。

`cc-select use <provider>` 会导出 `CLAUDE_CONFIG_DIR`，指向独立的 profile 目录（`~/.cc-select/profiles/<provider>/settings.json`）。Claude Code 启动时读取该目录的 env，从而实现“每个终端各自的服务商”。

## 安全说明

API key 目前以**明文**存储在 `~/.cc-select/profiles/<id>/settings.json` 中（文件权限 `0600`，目录权限 `0700`）。风险等级与 `~/.claude/settings.json` 相同。后续计划接入系统 Keychain；keychain 占位机制与 `internal/secrets` 包已实现，待接入 CLI/Web 写入路径。

## 构建

```bash
make all      # 构建前端 + 二进制，输出到 ./bin/cc-select
make test     # 运行 Go 单元测试
make vet      # 运行 go vet
make e2e      # 运行 Playwright 端到端测试
```

## 许可

Apache License 2.0
