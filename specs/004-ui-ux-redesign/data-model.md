# Data Model: UI/UX 改版与快速上手（v0.1.1）

**Date**: 2026-10-26 | **Spec**: [spec.md](./spec.md)

> 原则：`providers.json` / `profiles/<id>/settings.json` 数据模型**零变更**（Assumptions）。本文件只定义新增/演进的数据结构。

## 1. 存储层（Go）

### 1.1 `internal/presets/presets.json`（新，单源预设文件）

```jsonc
{
  "schemaVersion": 1,
  "presets": [
    {
      "id": "minimax",                    // 唯一，kebab-case
      "displayName": "MiniMax",
      "category": "cn_official",          // official|cn_official|aggregator|third_party|cloud_provider|custom
      "websiteURL": "https://minimax.io",
      "apiKeyURL": "https://platform.minimax.io",  // 可空（custom/official 可省）
      "apiFormat": "anthropic",           // anthropic|openai_chat|openai_responses|gemini_native
      "authField": "ANTHROPIC_AUTH_TOKEN",
      "envTemplate": {                    // ${VAR} 占位符 = 必填
        "ANTHROPIC_BASE_URL": "https://api.minimax.io/anthropic",
        "ANTHROPIC_AUTH_TOKEN": "${API_KEY}",
        "ANTHROPIC_MODEL": "MiniMax-M3",
        "ANTHROPIC_DEFAULT_HAIKU_MODEL": "MiniMax-M3.1-Flash",
        "ANTHROPIC_DEFAULT_SONNET_MODEL": "MiniMax-M3",
        "ANTHROPIC_DEFAULT_OPUS_MODEL": "MiniMax-M3"
      },
      "requiredVars": ["ANTHROPIC_AUTH_TOKEN"],
      "optionalVars": ["ANTHROPIC_MODEL"],
      "oauth": false
    }
    // … 既有 13 条迁入 + 通义千问 coding-plan 新条目；GLM 两条模型保鲜至 glm-5.3 系
  ]
}
```

**校验规则（`internal/presets/validate.go`，加载/拉取后强制）**：
- `schemaVersion` 必须等于编译期支持的版本（不匹配 → 整包拒绝）。
- `id` 唯一且非空；`displayName`/`category`/`apiFormat` 必填且取值合法。
- `envTemplate` 中每个 `${VAR}` 占位符必须被 `requiredVars ∪ optionalVars ∪ {API_KEY}` 之一声明。
- `requiredVars ⊆ envTemplate 的键 ∪ 认证字段`；认证字段（`ANTHROPIC_AUTH_TOKEN`/`ANTHROPIC_API_KEY`）必居其一（`oauth: true` 除外）。
- 所有 URL 若非空必须 `https://`。
- 违反任一条 → **整包拒绝**（不逐条剔除）。

### 1.2 `~/.cc-select/presets.cache.json`（新，远端预设缓存）

```jsonc
{
  "schemaVersion": 1,
  "fetchedAt": "2026-10-26T12:00:00Z",   // RFC3339
  "presets": [ /* 与 1.1 同构 */ ]
}
```

- 原子写（与 prefs/providers 同模式），0600。
- 损坏/缺字段 → 视为不存在，回落 builtin。

### 1.3 `prefs.json` 增量（向后兼容字段）

```jsonc
{
  "…既有字段不变…": {},
  "remotePresetsDisabled": false   // 新增；缺省 false = 启用远端（显式 true 才关闭）
}
```

- Go 侧 `Prefs.RemotePresetsDisabled bool \`json:"remotePresetsDisabled,omitempty"\``；旧版二进制读到新字段忽略，新版读到旧文件得零值——双向兼容（沿用 `DesktopPrefs` 先例）。

## 2. API 载荷（REST）

### 2.1 `GET /api/v1/presets`（响应增补，向后兼容）

```jsonc
{
  "presets": [ /* 既有 Preset 投影 */ ],
  "categories": ["official", "cn_official", "…"],
  "meta": { "source": "remote|cache|builtin", "fetchedAt": "RFC3339 | null" }   // 新增
}
```

### 2.2 `POST /api/v1/presets/refresh`（新）

```jsonc
// 请求：空 body
// 响应 200：
{ "meta": { "source": "remote", "fetchedAt": "…" } }
// 远端不可达/校验失败 → 200 + 当前生效源（不报 5xx，前端提示回落状态）：
{ "meta": { "source": "builtin", "fetchedAt": null }, "warning": "fetch_failed" }
```

### 2.3 `GET|PUT /api/v1/remote-presets`（新）

```jsonc
{ "enabled": true }   // GET 返回；PUT 全量替换（Prefs.RemotePresetsDisabled 反义）
```

### 2.4 `POST /api/v1/providers/probe`（新）

```jsonc
// 请求：向导/详情表单的最终 env（未保存也可测）
{ "env": { "ANTHROPIC_BASE_URL": "…", "ANTHROPIC_AUTH_TOKEN": "…", "ANTHROPIC_MODEL": "…" } }
// 响应 200：
{ "ok": true, "latencyMs": 412, "model": "glm-5.3" }            // model = 服务端回显（缺省回落请求值）
{ "ok": false, "latencyMs": 180, "error": { "kind": "auth", "message": "401 invalid api key" } }
// kind ∈ auth(401/403) | model(404/400-模型相关) | network(超时/DNS/连接) | server(5xx) | unknown
```

### 2.5 `POST /api/v1/usage/{providerId}`（新，仅手动）

```jsonc
// 响应 200：
{
  "supported": true,
  "providerId": "zhipu-glm",
  "fetchedAt": "2026-10-26T12:00:00Z",
  "windows": [
    { "type": "5h",   "used": 45, "limit": 100, "percent": 45, "resetAt": "2026-10-26T15:00:00Z" },
    { "type": "week", "used": 612, "limit": 2400, "percent": 25.5 },
    { "type": "5h",   "percent": 63 }        // GLM 事实：仅百分比，used/limit/resetAt 可缺省
  ]
}
// 不支持：
{ "supported": false, "providerId": "minimax", "fetchedAt": "…", "reason": "no_limit_headers" }
// type ∈ 5h | week | day | minute-requests | minute-tokens | month-mcp | balance
// 404：provider 不存在；500：内部错误（不吞）
```

## 3. 前端状态与导航模型

### 3.1 路由（hash）

| 路由 | 模块 | 可见性 |
|---|---|---|
| `#/providers`（默认） | 服务商（列表/画廊/用量） | 全部 |
| `#/mode-routes` | 模式与路由 | 全部 |
| `#/security` | 安全（Keychain） | 全部 |
| `#/desktop` | 桌面端 | 仅桌面形态（`/desktop/info` 200） |
| `#/general` | 外观与通用（主题/语言/远端预设/Shell 集成） | 全部 |
| `#/about` | 关于与更新 | 全部 |

### 3.2 前端目录（演进后）

```text
src/
├── App.tsx                  # 布局壳：侧边栏/顶 tab + 路由出口 + 全局 toast
├── navigation/              # hash router + 模块注册表（含桌面条件注册）
├── api/                     # 类型化客户端（presets/providers/probe/usage/remotePresets）
├── components/ui/           # shadcn 式基件（tailwind 重写，保留导出名兼容既有引用）
├── features/
│   ├── providers/           # ProviderList/ProviderCard + PresetGallery + ProviderWizard + UsagePanel
│   ├── mode-routes/         # GlobalModeCard + RoutesPanel（迁移）
│   ├── security/            # KeychainCard（迁移）
│   ├── desktop/             # DesktopSettings（迁移）
│   ├── general/             # 主题/语言设置 + RemotePresetsCard + ShellIntegration 设置区
│   └── about/               # 版本 + CheckUpdateButton + 使用说明
├── i18n/  theme/  presets/  # 既有，locale 增量
```

## 4. 实体关系小结

- `PresetBundle(1) ──包含──> Preset(N)`：builtin(embed)/cache/remote 三来源，整包生效。
- `Provider(既有) ──探测──> ProbeResult`：按即时 env，可不落盘。
- `Provider(既有) ──查询──> UsageReport`：按落盘 env，经 Adapter 注册表路由（host 匹配 → 专用适配器，否则通用头探测）。
