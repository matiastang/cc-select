# Contract: Web GUI REST 增量

> 既有 REST 面：`/api/v1/{providers,presets,mode,language,shell-integration,update}`（见 internal/web/api.go）。本特性只做增量，不改动既有端点语义。

## 1. `GET /api/v1/mode` / `PUT /api/v1/mode`（扩展）

- 值域扩展为 `settings-only | full | proxy`。
- `GET` 响应增加派生字段（便于前端展示模式说明）：

```json
{ "mode": "proxy" }
```

- `PUT` 载荷 `{"mode": "proxy"}` → 写 prefs.json（既有原子写路径）；**切换到/离开 proxy 触发的密钥迁移**（research D8）在此同步执行，迁移结果（成功条数/失败明细）随响应返回：

```json
{ "mode": "proxy", "migrated": 2, "failed": [] }
```

## 2. `GET /api/v1/routes`（新增）

- **行为**：列路由表 + daemon 状态（只读排障视图）。
- **响应**：

```json
{
  "router": { "running": true, "addr": "127.0.0.1:48270", "version": "0.0.6" },
  "routes": [
    { "tid": "ccs-3f9c2ab1", "provider": "minimax", "updatedAt": "2026-08-28T12:00:00Z" }
  ]
}
```

- tid 前端展示同 CLI 规则（短码）。

## 3. `DELETE /api/v1/routes`（新增，prune）

- **行为**：等价 `cc-select route prune`；可选 query `?olderThan=168h`。
- **响应**：`{"pruned": 3}`。

## 4. `POST /api/v1/router/ensure`（新增）

- **行为**：等价 `cc-select router ensure`（GUI「启动/修复路由服务」按钮）。
- **响应**：`{"running": true, "addr": "127.0.0.1:48270"}` 或 503 + 错误详情（FR-011 可诊断）。

## 5. 前端（React/TS）增量范围

- 模式选择器增加「proxy（代理路由）」选项 + 三模式说明文案（i18n en/zh）。
- 新增「活跃路由」面板：daemon 状态徽标、路由表、prune/ensure 按钮。
- provider 编辑表单无新增字段（`isolationMode` 下拉值域 +1）。

## 6. 不变式

- 列表/详情 DTO 的脱敏规则不变（api.go 顶部注释约定）：任何新端点不回显密钥明文或完整 tid。
- 遵循 TS `strict: true`，新增类型不落 `any`。
