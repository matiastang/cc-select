# Contract: 桌面模式 HTTP API（desktop-client）

**Date**: 2026-10-26 | **Spec**: [spec.md](../spec.md) | **Data Model**: [data-model.md](../data-model.md)

> 范围：桌面 App 内 WebView 发出的请求契约。**既有 `/api/v1/*` 全部端点契约零改动**（桥接层透传）；本契约只定义「拦截改写」与「新增」两类。

## 1. 请求路由总规则（桥接层，desktop/bridge.go）

```text
请求进入 desktop AssetServer Handler：
├── path 以 /api/v1/update 开头  → 桌面更新桥（§2，改写语义）
├── path 以 /api/v1/desktop 开头 → 桌面专属端点（§3，新增）
├── path 以 /api/v1/ 开头（其余）→ 透传 internal/web.Handler()（零改动）
└── 其他（非 /api/v1/ 前缀）      → 404 JSON（MUST NOT 返回 HTML/吞 SPA fallback）
```

**错误契约（全端点统一）**：`{"error": "<i18n 键或消息>"}` + 语义化状态码；非 `/api/v1/` 前缀的进入请求一律 `404 {"error":"not found"}`——这是 research R1 已知坑的硬性防线，有单测锁定（updatebridge_test.go）。

## 2. 更新端点拦截（既有端点的桌面语义改写）

### 2.1 `GET /api/v1/update/check`

| | |
|---|---|
| 桌面行为 | 调 Wails Updater（GitHub provider）检查 → **映射为与 web 端点同构的响应体**（前端 CheckUpdateButton 零改动） |
| 成功 | `200` `{ current, latest, updateAvailable, notes? }`（字段名以现有 web 实现为准，桥接层负责映射，差异在单测中锁定） |
| 离线/失败 | `200` `{ current, latest: current, updateAvailable: false, error?: "check failed" }`——**温和降级**，不抛 5xx 打断界面（FR-006 验收 3） |
| 与 web 差异 | web 端点查 `internal/updater`（CLI 仓库信息）；桌面端点查 App 自身版本——同一 URL、两种形态各自的正确语义 |

### 2.2 `POST /api/v1/update`

| | |
|---|---|
| 桌面行为 | 受理后进入 Wails Updater headless 安装流：下载 → SHA256SUMS 校验 → 原子替换（helper 进程，失败自动回滚）→ 引导重启 |
| 受理 | `202` `{ status: "downloading" }`；进度沿用前端既有进度通道语义（轮询/SSE 同构映射） |
| 并发防护 | 沿用 web 端点的串行锁语义：进行中再次 POST → `409 {"error":"update in progress"}` |
| 校验失败 | `422 {"error":"checksum mismatch"}`，当前版本不受影响（FR-007 验收 2/4） |
| 中断恢复 | 更新中断不产生 API 状态残留；下次 check 正常（Updater 备份/回滚机制负责，端点无状态） |

**红线**：桌面模式下该端点 **MUST NOT** 调用 `internal/updater` 的二进制同路径替换逻辑（那会替换 App 自身路径的假设不成立，见 research R2）。

## 3. 桌面专属端点（新增）

### 3.1 `GET /api/v1/desktop/info` — 能力探测 + 状态聚合

`200`：

```json
{ "mode": "desktop",
  "appVersion": "0.1.0",
  "embeddedCliVersion": "0.1.0",
  "cli": { "installedPath": "/usr/local/bin/cc-select",
           "installedVersion": "0.0.7",
           "updateAvailable": true } }
```

- `cli.installedPath == null` → 未安装（前端展示「安装到 PATH」主按钮）
- `installedVersion == "unknown"` → 外部/无法识别版本（覆盖前必须确认，见 3.3）
- **web 模式（`ccs gui`）下该端点不存在**（404）——前端以 404 为「非桌面」信号，隐藏 Desktop 设置区（FR-015 条件渲染契约，vitest 锁定）

### 3.2 `GET / PUT /api/v1/desktop/settings` — 桌面偏好

- GET `200`：`{ "trayResident": true, "autoStart": false, "autoUpdateCheck": true }`
- PUT 同构 body：
  - `204` 成功（落盘 prefs.json 原子写 + 同步 Wails autostart/tray 服务即时生效）
  - `400 {"error":"invalid field"}` 非布尔/未知字段
  - `503 {"error":"autostart unavailable"}` OS 层自启注册失败（如企业策略禁用）——**prefs 不落盘已改字段**（先服务后落盘，失败整体回滚，避免 UI 与实际不一致）
- 幂等：重复 PUT 相同 body → 204，无副作用

### 3.3 `POST /api/v1/desktop/cli/install` — 安装内置 CLI 到 PATH

请求：`{ "confirmOverwrite": false }`

| 场景 | 响应 |
|---|---|
| 干净安装 / 同版本重装 | `200 { "installedPath": "...", "installedVersion": "0.1.0", "action": "installed" \| "reinstalled" }` |
| 存在旧版本（可识别为本工具） | `200 { ..., "action": "upgraded" }`（同版本幂等为 reinstalled） |
| 目标为外部/无法识别的二进制 | `409 { "conflict": "external-binary", "installedPath": "...", "installedVersion": "unknown" }` → 前端确认后带 `confirmOverwrite: true` 重发 → 覆盖 |
| 目标目录无写权限 | `409 { "conflict": "permission", "hint": "<平台指引 i18n 键>" }`；macOS/Linux 自动回落尝试 `~/.bin`（回落成功则 200 + `hint` 提示需将该目录加入 PATH） |
| 平台 | mac/linux：`/usr/local/bin`（回落 `~/.bin`）；windows：`%LOCALAPPDATA%\Programs\cc-select\` + 写 HKCU `Path`（无需管理员） |

### 3.4 `GET /api/v1/desktop/tray/commands` — 托盘动作语义（只读）

`200`：`[ { "providerId": "glm", "label": "GLM", "copyCommand": "ccs use glm" }, ... ]`

- 托盘菜单数据源（research R5）：点击 provider = 复制 `copyCommand` + 系统通知；**不存在任何「执行切换」的动作端点**——R1 红线在契约层的形式化（FR-009）
- 该端点便于对菜单数据做单测（不依赖真实托盘）

## 4. 兼容性承诺

- 本契约全部端点仅桌面模式存在/改写；web 模式（`ccs gui`）行为与今日完全一致（既有 api_test.go / server_test.go 全绿即证明）
- `/api/v1/update*` 响应体结构与 web 端点保持同构演进：字段变更必须两侧同 PR 修改并同步测试（FR-015 的工程化表达）
