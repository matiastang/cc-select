# Contract: REST API 增量（v0.1.1）

**Spec**: [spec.md](../spec.md) | FR-012–FR-020 | 变更面：`internal/web/api.go` + 新包 `internal/probe`、`internal/usage`

## 1. 既有端点（增补响应，向后兼容）

| 端点 | 变更 |
|---|---|
| `GET /api/v1/presets` | 响应新增 `meta{source, fetchedAt}`；`presets`/`categories` 结构不变（旧前端忽略新字段） |
| `GET /api/v1/presets/{id}` | 不变 |

`source` 语义：`remote`（本次会话成功拉取过远端，或缓存 <24h 且来自远端）/ `cache`（缓存存在但已过期，仍采用）/ `builtin`（embed 兜底）。`fetchedAt` 为远端最近成功拉取时间，builtin 时为 null。

## 2. 新端点

### 2.1 `POST /api/v1/presets/refresh` — 手动刷新预设

- 权限：本地 GUI（既有本地服务边界，无新增鉴权面）。
- 行为：忽略 TTL 立即拉取远端；成功 → 校验 → 写缓存 → 返回新 meta；失败/校验拒绝 → 不动缓存，返回**当前生效源** + `warning: "fetch_failed" | "invalid_schema" | "disabled"`。
- 永远 200（网络失败不是服务端错误，前端据 warning 提示）。
- `remotePresetsDisabled = true` 时：不发起网络请求，返回 `warning: "disabled"` + builtin meta。

### 2.2 `GET|PUT /api/v1/remote-presets` — 远端预设开关

- GET → `{enabled: bool}`（`!Prefs.RemotePresetsDisabled`）。
- PUT 全量 `{enabled}` → 写 prefs；非法 body → 400 `{"error": "…"}`。
- PUT false 后：内存预设源立即回落 builtin（下次读取即生效），缓存文件保留（再开启时若未过期直接复用）。

### 2.3 `POST /api/v1/providers/probe` — 连通性测试（FR-016/017）

- 请求：`{"env": {"ANTHROPIC_BASE_URL": "https://…", "ANTHROPIC_AUTH_TOKEN|ANTHROPIC_API_KEY": "…", "ANTHROPIC_MODEL": "…", …}}`。
  - `ANTHROPIC_BASE_URL` 缺省 → 探测官方 `https://api.anthropic.com`。
  - 认证字段二者至少其一，否则 400 `{"error": "missing_auth"}`。
  - 模型缺省 → `claude-sonnet-4-5`（官方目录安全值；带 base URL 时要求显式模型，缺失 400 `{"error": "missing_model"}`——第三方端点无兜底目录）。
- 行为：`POST {base}/v1/messages`，body `{"model": M, "max_tokens": 1, "messages": [{"role": "user", "content": "ping"}]}`；超时 10s；**不重试**。
- 响应 200 `{"ok": true, "latencyMs": n, "model": "服务端回显|请求值"}` 或 `{"ok": false, "latencyMs": n, "error": {"kind": "…", "message": "≤200 字符摘要"}}`。
- `kind` 判定表：HTTP 401/403 → `auth`；404/400 且响应体含 model 相关错误 → `model`；`net.Error` 超时/DNS/连接拒绝 → `network`；5xx → `server`；其余 → `unknown`。
- 副作用约束：**仅由用户点击触发**（前端契约）；后端不做任何自动调用。

### 2.4 `POST /api/v1/usage/{providerId}` — 限额查询（FR-018/019/020）

- 行为：读该 provider 落盘 env → Adapter 注册表按 `ANTHROPIC_BASE_URL` host 匹配（`open.bigmodel.cn`/`api.z.ai` → GLM 适配器；`api.moonshot.cn`/`api.moonshot.com` → Kimi 余额适配器；其余含 base URL → 通用头探测；无 base URL → 官方头探测 `api.anthropic.com`）→ 执行查询（超时 10s，不重试）。
- 响应：见 data-model §2.5。`supported: false` 是**正常结果**（200），`reason` ∈ `no_limit_headers` | `no_public_endpoint`。
- 404：provider 不存在。500：仅内部错误（适配器 HTTP 失败一律归一为 200 + windows 内 error 窗口或顶层 `error` 字符串——**不吞 panic**，错误消息不含 token）。
- 每次调用即实时查询（无服务端缓存——FR-019 仅手动刷新，前端展示自带 fetchedAt）。

## 3. 错误体统一

沿用既有 `{"error": "message"}` 形状；新端点补充字段一律加在顶层，不嵌套 error。

## 4. 桌面端

Wails 桌面经 `web.Handler()` 复用以上全部端点（desktop/main.go 桥不感知本契约）；`/desktop/*` 命名空间不变。
