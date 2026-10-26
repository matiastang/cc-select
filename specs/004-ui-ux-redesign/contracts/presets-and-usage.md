# Contract: 预设单源化与用量适配器（v0.1.1）

**Spec**: [spec.md](../spec.md) FR-010–FR-015, FR-018–FR-020 | research §2.2/2.3/3.2/3.3

## 1. 预设单源（`internal/presets` 重构）

### 1.1 文件与分发

- 唯一真值文件：`internal/presets/presets.json`（schema 见 data-model §1.1），`//go:embed presets.json` 为兜底源。
- 远端地址（编译期常量，MUST NOT 可被配置覆盖——安全约束 FR-014）：
  `https://raw.githubusercontent.com/matiastang/cc-select/main/internal/presets/presets.json`
- CI 校验：`make check` 增加 `presets-lint` 步骤——以与运行时同一校验器检查仓库内文件，改坏预设无法过 CI。

### 1.2 源解析状态机（`source.go`）

```
Read(ctx):
  if prefs.RemotePresetsDisabled → builtin(meta: builtin)
  if 内存已解析(本次进程内) → 返回
  if cache 存在且 fetchedAt + 24h > now → cache(meta: remote*, fetchedAt)
  fetchRemote(ctx, 5s timeout, HTTPS, pinned URL):
    ok   → validate → 写缓存(原子) → 内存态 = remote
    fail → cache 存在(任意年龄) → cache(meta: cache, fetchedAt)
         → builtin(meta: builtin)
Refresh(ctx): 忽略 TTL 强制 fetchRemote；成功/失败同上，返回 meta + warning
```

- 并发：`sync.Once`/mutex 保护进程内解析；Web 与 CLI 每进程各解析一次。
- 校验失败与网络失败同路径（回落），`warning` 区分 `fetch_failed | invalid_schema`。
- **对外 API 不变**：`All()/AllByCategory()/ByID()/BuildEnv()/Categories()/FormatMissing()` 签名保持，CLI `add` 与 Web handler 零改动即获远端能力（FR-015）。新增 `Meta() SourceMeta`。

### 1.3 首批预设清单（数据事实锁定，research §2.1/2.2）

| id | 动作 | 关键 env |
|---|---|---|
| `minimax`（新） | 新增 | base `https://api.minimax.io/anthropic`；主/Sonnet/Opus `MiniMax-M3`、Haiku `MiniMax-M3.1-Flash` |
| `qwen-coding`（新） | 新增 | base `https://coding.dashscope.aliyuncs.com/apps/anthropic`；全档 `qwen3.7-plus` |
| `zhipu-glm` / `zhipu-glm-en` | 模型保鲜 | 主/Sonnet/Opus `glm-5.3`、Haiku `glm-5.3-flash` |
| 其余 11 条 | 迁入 JSON 原样 | — |

## 2. 用量适配器（`internal/usage`）

### 2.1 接口与注册表

```go
type Report struct {
    Supported  bool
    Windows    []Window   // 归一化窗口
    FetchedAt  time.Time
}
type Window struct {
    Type     string     // 5h|week|day|minute-requests|minute-tokens|month-mcp|balance
    Used, Limit float64 // 任一可缺省（指针/0 值 + Has 标记由实现定）
    Percent  *float64   // GLM 仅百分比
    ResetAt  *time.Time
}
type Adapter interface {
    Match(baseURL string) bool          // 按 host 匹配
    Query(ctx context.Context, env map[string]string) (*Report, error)
}
```

- 注册表按声明序匹配，首个命中生效；**默认兜底适配器 = 通用头探测**（恒 Match）。
- token 从 env 的认证字段读取；错误消息 MUST NOT 包含 token。

### 2.2 适配器实现矩阵（首批）

| 适配器 | 匹配 host | 机制 | 输出窗口 |
|---|---|---|---|
| `anthropicHeaders`（兜底） | 恒匹配 | `POST {base}/v1/messages`（max_tokens=1）→ 解析 `anthropic-ratelimit-unified-*`（5h/week，多窗口条目与独立头两种形态都认）+ `anthropic-ratelimit-requests/tokens-*`（minute-*）；零窗口 → `Supported:false, reason:no_limit_headers` | 5h/week/minute-* |
| `glmQuota` | `open.bigmodel.cn`、`api.z.ai` | `GET {scheme}://{host}/api/monitor/usage/quota/limit`，`Authorization: <token>`（无 Bearer）；解析 `data.limits[]`：`TOKENS_LIMIT`→5h（percent）、`TIME_LIMIT`→month-mcp（percent） | 5h、month-mcp |
| `kimiBalance` | `api.moonshot.cn`、`api.moonshot.com` | `GET {base}/v1/users/me/balance`，`Authorization: Bearer <token>`；余额窗口 type=balance | balance |
| （无）`api.kimi.com` | 不注册 | 无公开端点 → 落兜底头探测，预期 no_limit_headers → unsupported（诚实呈现） | — |

### 2.3 共享客户端（与 `internal/probe` 共用）

`internal/probe/client.go` 提供：`SendMessage(ctx, baseURL, token, authHeader, model, body) (status, header, body, elapsed, err)` + 错误分类（auth/network/model/server/unknown）。`probe` 与 `usage.anthropicHeaders` 都走它——单点维护超时（10s）与分类逻辑。

## 3. 安全审查要点（宪法 VI）

- 远端预设 URL 硬编码 + `https` 校验双保险；不跟随重定向到其他 host（`http.Client` 自定义 `CheckRedirect` 拒绝跨 host）。
- probe/usage 的错误透传不含请求头/体原文（截断 200 字符 + 剥 token 子串）。
- 缓存/prefs 原子写 0600（复用既有原子写工具）。
