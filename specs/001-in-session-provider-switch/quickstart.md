# Quickstart: 会话内切换 provider 验证指南

> 端到端人工验证场景（自动化测试在 tasks.md 定义，此处是「上手即可跑」的证明路径）。引用：[CLI 契约](./contracts/cli.md)、[HTTP 契约](./contracts/router-http.md)、[数据模型](./data-model.md)。

## 前置

- `make all` 构建最新 `./bin/cc-select`，已 `cc-select init`（`ccs` 可用）。
- 已配置两个可用的第三方 provider（例：`glm`、`minimax`），当前处于默认 shell（zsh/bash 或 PowerShell）。
- 真机验证需真实可用额度；CI/无额度场景用 §4 的自动化链路替代。

## 1. P0：跨 provider 续会话（不依赖新组件）

```bash
ccs use glm
claude                     # 随便聊几轮，让 Claude 记住一个"暗号"（如：我叫这个项目 pineapple）
# Ctrl+D 退出
ccs use minimax
claude --continue          # 问：我给这个项目起的暗号是什么？
```

**预期**：MiniMax 能答出 `pineapple`（上下文跨 provider 延续，FR-005/SC-004）；`cc-select current` 显示 minimax。
**Mode A 用户**：文档明示此流程不适用（isolation-modes.md）。

## 2. P1：会话内热切（核心场景）

```bash
cc-select mode proxy       # 或 ccs use glm --mode proxy 首次试用
ccs use glm
echo $CC_SELECT_TID        # 应形如 ccs-3f9c…（每 shell 一次）
cc-select router status    # router=ok
claude                     # 正常对话（此时经代理走 GLM）
```

保持 claude 运行，在**会话内**让 Claude 执行（或 `!` 自跑）：

```bash
cc-select route switch minimax
```

然后继续对话（可先问「我给项目起的暗号是什么？」验证上下文仍在）。

**预期**：
- 切换命令输出 `glm → minimax`，耗时 <1s（SC-001/SC-006）；
- 后续回复实际由 MiniMax 服务（`cc-select current` 显示 minimax，与切换一致，FR-008）；
- claude 全程未重启（FR-001），对话历史延续（FR-003）；
- 请求路径可由 daemon 前台日志核对：`cc-select router stop && cc-select router serve --foreground`（另开终端重复上述流程，观察 `tid=… provider=…` 行）。

## 3. 隔离与失败语义

```bash
# 终端 B（新开一个窗口）
ccs use deepseek           # 或任一第三家
claude                     # 持续对话中……
# 回到终端 A 的会话内反复切换：
cc-select route switch minimax && cc-select route switch glm   # 往复 ≥10 次
# 终端 B 继续对话
```

**预期**：终端 B 始终走 deepseek（`route status` 与实际响应不变，FR-004/SC-003）。

```bash
cc-select route switch nosuchprovider   # 失败路径
cc-select route switch glm              # 随后正常切回 → 会话无残留影响（FR-007）
cc-select router stop                   # 模拟 daemon 崩溃
cc-select route status                  # router=down + 自愈指引
cc-select router ensure                 # 自愈
# claude 里继续对话 → 恢复，未重开会话（FR-011）
```

## 4. 无真实额度 / 回归环境：自动化链路

```bash
go test ./internal/routes/... ./internal/router/... -tags integration
```

**覆盖**：fake upstream 断言每笔请求的路由归属（tid → provider）、model 改写、伪 token 鉴权、双 TID 交叉切换 ≥10 次零串扰（D13，SC-003 脚本化）。
GUI 面：`make e2e`（模式选择含 proxy、路由面板渲染）。

## 5. 安全核对（US4）

```bash
cc-select mode proxy       # 触发迁移后
grep -r "ANTHROPIC_AUTH_TOKEN" ~/.cc-select/providers.json ~/.cc-select/profiles/
# 预期：providers.json 中为 $keychain:cc-select:<id>:ANTHROPIC_AUTH_TOKEN 占位；
#       profiles/*/settings.json 不含任何真实密钥（FR-009）
```
