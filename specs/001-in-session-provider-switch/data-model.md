# Data Model: 会话内切换 provider（Phase 1）

> 实体来自 [spec.md Key Entities](./spec.md)；本文件落到可实现的字段与状态机层面。既有实体（Provider、Prefs）只列增量。

## 1. TerminalSession（终端会话）—— 概念实体，无持久化

一个 shell 及其派生的 Claude Code 会话的集合；身份由 shell env 承载，随 shell 进程消亡。

| 字段 | 载体 | 规则 |
|---|---|---|
| `tid` | `CC_SELECT_TID`（shell env） | `ccs-` + 128bit 随机 hex（如 `ccs-3f9c2ab1...`）；每 shell 生成一次（`OpSetIfUnset` 守卫）；= 伪 token 值（`ANTHROPIC_AUTH_TOKEN`） |
| `activeProvider`（记录副本） | `CC_SELECT_ACTIVE`（shell env） | 启动时由 `use` 写入；**会话内切换后滞后**，仅作非 Mode P 的展示/兼容 |
| `mode` | 由 prefs 三级解析决定 | 该终端 `use` 时的模式快照（决定发射内容） |

**状态机**：

```text
[无 TID] --ccs use X (proxy)--> [TID 已建 + route=X + profile(X) 指向 + BASE_URL=router]
[TID 已建] --cc-select route switch Y--> [route=Y；env 不变]
[TID 已建] --ccs use Z (proxy)--> [TID 保持 + route=Z + profile(Z) 指向]
[shell 退出] --> [TID 消亡；routes.json 留孤儿条目（prune 收敛）]
```

## 2. RouteEntry（路由条目）—— `~/.cc-select/routes.json`

```json
{
  "version": 1,
  "routes": [
    { "tid": "ccs-3f9c...", "provider": "minimax", "updatedAt": "2026-08-28T12:00:00Z" }
  ]
}
```

| 字段 | 类型 | 校验/规则 |
|---|---|---|
| `tid` | string | 主键；格式 `^ccs-[0-9a-f]{32}$` |
| `provider` | string | 必须存在于 providers.json（写入前校验，原子性：校验失败不落盘）；v1 禁 `claude-official`（research D2） |
| `updatedAt` | RFC3339 | 每次写入刷新；prune 依据 |

**状态转移**：`set`（use，无则建）/ `switch`（route switch，原子替换 provider+updatedAt）/ `prune`（按过期阈值删除）。写：0600 + 临时文件 rename（同 providers.json 约定）。读：daemon 每请求、`current`/`route status` 按需。

## 3. RouterState（daemon 状态）—— `~/.cc-select/router.json`

```json
{
  "addr": "127.0.0.1:48270",
  "pid": 12345,
  "startedAt": "2026-08-28T12:00:00Z",
  "version": "0.0.6"
}
```

| 字段 | 类型 | 规则 |
|---|---|---|
| `addr` | string | 监听地址；优先级：状态文件 > `CC_SELECT_PROXY_ADDR` > 默认 `127.0.0.1:48270`。**重启必须沿用**（claude BASE_URL 已固化） |
| `pid` | int | 仅诊断用；存活判定以 healthz 为准（PID 复用不可靠） |
| `version` | string | daemon 自身版本；healthz 回显，版本不匹配时 ensureDaemon 重启升级 |

**生命周期**：`ensure`（healthz 探测 → 健康复用 / 死亡则 detached 重启）/ `stop`（healthz 侧信道关闭或 PID 终止，幂等）。

## 4. Provider（既有实体，增量）

| 增量 | 说明 |
|---|---|
| `IsolationMode` 新值 `proxy` | per-provider 覆盖可选 Mode P；`prefs.Valid()` 扩枚举 |
| `Env` 值可为 `$keychain:cc-select:<id>:<var>` 占位 | **既有机制**；Mode P 启用时迁移触发其实际使用（research D8）。daemon 侧解析 + 内存缓存 |
| 约束（Mode P 下） | `Env` 中的 `ANTHROPIC_BASE_URL`/`ANTHROPIC_AUTH_TOKEN` 不再写入 profile settings.json（由 profile env 模板与 shell TID 取代）；`ANTHROPIC_MODEL` 仅作 daemon 改写映射源 |

## 5. Profile settings.json（Mode P 变体）

```json
{
  "env": { "ANTHROPIC_BASE_URL": "http://127.0.0.1:48270" }
}
```

- 其余结构（全局 settings 合并、共享链接白名单）沿用 Mode B；`projects/` 等仍链接回 `~/.claude`（上下文延续的物质基础）。
- **不含任何密钥**（US4）；不含 `ANTHROPIC_AUTH_TOKEN`（留给 shell TID）。

## 6. Prefs（既有实体，增量）

`IsolationMode` 枚举增加 `proxy`；其余字段不变。

## 7. 实体关系

```text
Provider (providers.json)
   ▲ provider 引用（写入前校验）
RouteEntry (routes.json) ──tid──▶ TerminalSession (shell env: CC_SELECT_TID)
                                        │ 启动时固化
                                        ▼
                              Claude Code 进程 (BASE_URL=router, AUTH_TOKEN=tid)
                                        │ 每笔请求携带 tid
                                        ▼
                              Router daemon (router.json 状态)
                                ├─ 查 routes: tid → provider
                                ├─ 解析 provider env（keychain 占位 → 真值，内存缓存）
                                └─ 改写 model → 转发上游
```

**不变式**：
- 任一 tid 任一时刻在 routes.json 中至多一条（主键）；
- FR-004（隔离）的不变式基础：请求归属仅由 tid 决定，tid 与 shell 一一对应（守卫生成）；
- FR-007（原子性）的不变式基础：`switch` 是单文件原子替换，无中间态可见。
