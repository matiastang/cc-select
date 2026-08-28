# Contract: CLI 命令（Mode P 增量）

> 面向用户与脚本的稳定接口。所有新输出走 i18n（en/zh）；退出码：0 成功、1 一般失败、2 用法错误（沿既有约定）。

## 1. `cc-select route` —— 路由表操作（会话内切换的触发入口）

### `cc-select route switch <providerID>`

- **行为**：解析本终端 TID（读 env `CC_SELECT_TID`，或 `--tid` 显式指定）→ 校验 provider 存在且非 `claude-official`（v1）→ 原子写 `routes.json`。**普通命令：不输出 eval 语句，不改父 shell env**——因此可在 Claude Code 的 Bash 工具内直接执行（宪法 I 边界内）。
- **输出（stdout）**：`<old> → <new>`（old 无则 `∅ → <new>`）；stderr 打印诊断/告警。
- **失败语义（FR-007）**：provider 不存在 / TID 缺失（且未 `--tid`）/ 表写入失败 → 非零退出，路由表不变。
- **退出后效应（FR-002）**：daemon 每请求重读路由表 → 下一次模型请求即生效；在途请求按原 provider 完成。

### `cc-select route list`

- **输出**：全部 `tid 短码 | provider | updatedAt`（tid 打码显示前 12 字符）。

### `cc-select route status`

- **行为**：读 `CC_SELECT_TID`（可 `--tid`）→ 显示本终端当前路由 + daemon 存活状态（healthz）。
- **输出**：`tid=ccs-3f9c… provider=minimax router=ok(127.0.0.1:48270)`；daemon 不活时 `router=down`（附自愈指引）。

### `cc-select route prune [--older-than 168h]`

- **行为**：删除 `updatedAt` 早于阈值的条目；默认 7d。输出删除数量。

## 2. `cc-select router` —— daemon 生命周期

### `cc-select router ensure`

- **行为**：healthz 探测 → 健康复用 / 否则 detached 拉起 `router serve` 并等 healthz（≤2s）。`use`/`route`（proxy 模式）内部自动调用。幂等。

### `cc-select router serve [--foreground]`

- **行为**：daemon 入口。默认 detached（内部经 ensure 调用）；`--foreground` 供排障（日志到 stderr）。监听 addr 解析优先级见 data-model §3。

### `cc-select router stop`

- **行为**：通知 daemon 退出（healthz 侧信道），幂等。用户主动停用 Mode P 排障用。

## 3. `cc-select use <provider>`（proxy 模式下的发射契约）

模式为 `proxy`（`--mode proxy` > provider 覆盖 > 全局）时，除既有流程（profile.Sync 幂等重建 + 共享链接）外：

1. **前置**：`router ensure`（失败即中止并给恢复指引，FR-011）；写 `routes.json[tid]=<id>`。
2. **profile**：settings.json `env` 仅 `{ANTHROPIC_BASE_URL: http://<router addr>}`（D5，屏蔽全局 env 污染）。
3. **发射语句**（stdout，供 `ccs()` eval；zsh/bash 形态）：

```sh
if [ -z "${CC_SELECT_TID:-}" ]; then
  export CC_SELECT_TID='ccs-3f9c2ab1d0e4f5a6b7c8d9e0f1a2b3c'
fi
export ANTHROPIC_AUTH_TOKEN="$CC_SELECT_TID"
export CLAUDE_CONFIG_DIR='/Users/x/.cc-select/profiles/minimax'
export CC_SELECT_ACTIVE='minimax'
```

   PowerShell 等价（`if (-not $env:CC_SELECT_TID) { $env:CC_SELECT_TID = '...' }`）。注意：`ANTHROPIC_AUTH_TOKEN` 引用 `$CC_SELECT_TID` 而非字面值——TID 守卫先行保证求值顺序。
4. **切换到官方 provider**：Mode P 不适用（D2）——官方目标时回退既有发射并提示。

## 4. `cc-select current`（Mode P 语义增量）

- env `CC_SELECT_TID` 存在且 `routes.json` 有该条目 → **以路由表为真值**输出（FR-008；`CC_SELECT_ACTIVE` 可能滞后于会话内切换）。
- 无 TID / 无条目 → 既有行为（读 `CC_SELECT_ACTIVE`）不变。

## 5. `cc-select mode`

- 参数域扩展：`settings-only | full | proxy`；`GET`/`SET` 行为不变。GUI 同步（web-api.md）。

## 6. 环境变量契约（新增汇总）

| 变量 | 写入者 | 语义 | 生命周期 |
|---|---|---|---|
| `CC_SELECT_TID` | `use`（守卫式，仅首次） | 终端身份 = 伪 token | shell 进程 |
| `CC_SELECT_PROXY_ADDR` | 用户 | daemon 监听地址覆盖（仅首次启动生效，之后以 router.json 为准） | 用户设置 |
