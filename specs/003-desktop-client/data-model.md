# Data Model: Wails v3 桌面客户端（desktop-client）

**Date**: 2026-10-26 | **Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

> 原则：**不新建存储**。既有实体（Provider/Profile/Prefs/Routes）零改动；本特性只新增/扩展下表所列的少量结构与纯运行时状态。

---

## 1. 持久化实体

### 1.1 DesktopPrefs（扩展 `internal/prefs.Prefs`）

挂在既有 `prefs.json` 的 `Prefs` 上，新增可空子结构 `Desktop *DesktopPrefs`。旧版 CLI/App 读到未知字段忽略，新版读到缺失字段取零值后**归一化为默认**——双向向后兼容（Edge Case「版本不一致」在偏好层的落地）。

| 字段 | 类型 | 默认 | 约束 / 校验 |
|---|---|---|---|
| `trayResident` | bool | `true` | 关窗驻留托盘 vs 关窗退出（FR-008）；Save 前经 `Normalize()` |
| `autoStart` | bool | `false` | 登录自启（FR-010）；与 Wails Autostart 服务同步（写 prefs 成功后再调服务，失败回滚显示态） |
| `autoUpdateCheck` | bool | `true` | 启动时静默检查更新（FR-006） |

**状态迁移**：`(nil) → 首次保存`（任意开关被用户改动时创建并整体落盘）；无其他迁移。原子写沿用 prefs.Save 既有实现。

**校验规则**：仅布尔字段，无格式校验；非法类型由 `encoding/json` 解码报错接管（与既有 prefs 行为一致）。

### 1.2 CLI 安装状态（派生，不落盘）

安装器运行时探测结果，喂给前端展示；**不持久化**（每次打开设置区实时探测，天然反映外部变化——Edge Case「App 运行中终端发生变更」同类语义）。

| 字段 | 类型 | 说明 |
|---|---|---|
| `embeddedVersion` | string | App 内置 CLI 版本（构建期注入，常量） |
| `installedPath` | string? | `exec.LookPath("cc-select")` 结果；空 = 未安装 |
| `installedVersion` | string? | 已安装二进制 `--version` 输出；探测失败 = `"unknown"` |
| `updateAvailable` | bool | `installed != embedded` 且 installed 存在 |

**状态迁移**（安装按钮状态机）：`not-installed → installed(same) → installed(stale)`；`installed(external)`（路径存在但 `--version` 非本工具输出）需用户确认才覆盖（FR-004/FR-005 验收 2/4）。

---

## 2. 传输实体（DTO，仅桌面模式新增端点）

### 2.1 DesktopInfoDTO — `GET /api/v1/desktop/info`

```json
{ "mode": "desktop", "appVersion": "0.1.0", "embeddedCliVersion": "0.1.0",
  "cli": { "installedPath": "/usr/local/bin/cc-select", "installedVersion": "0.0.7", "updateAvailable": true } }
```

前端以该端点**探测桌面能力**：200 → 渲染 Desktop 设置区；404（web 模式）→ 整区隐藏（FR-015 条件渲染契约）。

### 2.2 DesktopSettingsDTO — `GET/PUT /api/v1/desktop/settings`

GET 返回 DesktopPrefs 三开关；PUT 接受同构 body，服务端 `Normalize()` 后原子落盘并同步 Wails 服务（autostart/tray 行为即时生效）。

### 2.3 桌面模式更新响应（拦截 `/api/v1/update*` 的出参）

保持与既有 web 端点**同构**（前端卡片零改动）：

- `GET /api/v1/update/check` → `{ "latest": "0.2.0", "current": "0.1.0", "notes": "...", "updateAvailable": true }`（字段名以现有实现为准，桥接层做字段映射）
- `POST /api/v1/update` → 受理后由 Wails Updater headless 执行：下载 → SHA256SUMS 校验 → 原子替换 → 提示重启；进度经 SSE/轮询沿现有前端进度通道（同构映射）；失败返回与现有错误契约一致的结构（见 contracts/desktop-api.md）

---

## 3. 发布实体（构建期，非运行时）

### 3.1 桌面 Release 资产（每 tag 三平台）

| 资产 | 平台 | Updater 可热更 |
|---|---|---|
| `cc-select-desktop_<ver>_darwin_arm64.zip`（内含 .app） | macOS | ✅（zip 格式是 updater 硬要求） |
| `cc-select-desktop_<ver>_windows_x64.zip`（内含 .exe 安装器） | Windows | ✅ |
| `cc-select-desktop_<ver>_linux_x64.AppImage` | Linux | ✅（$APPIMAGE 原地替换） |
| `SHA256SUMS`（全平台合并，含 CLI 资产） | 全部 | 校验源 |

命名/校验细则见 [contracts/release-assets.md](./contracts/release-assets.md)。

---

## 4. 实体关系图（增量视角）

```text
prefs.json (既有)
└── Prefs (既有) ──新增字段──▶ DesktopPrefs (1:1, 可空)

~/.cc-select/ 两层存储 (Provider/Profile/Routes)  ── 零改动，App 与 CLI 共享

运行时派生：CliInstallState ──探测── exec.LookPath + --version
发布期实体：Release 资产 + SHA256SUMS ──消费── Wails Updater (GitHub provider)
```

## 5. 验证规则汇总（对应 spec 验收）

- DesktopPrefs 落盘后旧版 CLI 读取不报错、字段不丢（版本共存 Edge Case）
- CliInstallState 四态驱动安装按钮四文案：安装 / 已是最新 / 升级到 vX / 外部版本确认覆盖
- 更新中断 → 重新启动仍为旧版且可再次发起（Updater 原子性 + 备份回滚，FR-007 验收 4）
