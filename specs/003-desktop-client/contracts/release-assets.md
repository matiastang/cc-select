# Contract: 发布产物与更新通道（desktop-client）

**Date**: 2026-10-26 | **Spec**: [spec.md](../spec.md) | **Data Model**: [data-model.md](../data-model.md) §3

> 范围：v0.1.0 起 GitHub Release 的**桌面端资产命名与校验契约**。CLI 资产沿用现有命名不变；本契约是 Wails Updater GitHub provider 正确匹配与校验的依据（research R2/R7）。

## 1. 版本与触发

- **单一 tag 双产物线**：既有 release tag（如 `v0.1.0`）同时产出 CLI 资产（不变）与桌面端资产（本契约），版本号强制同源——避免 App 内置 CLI 与 App 版本漂移。
- 桌面端资产仅在 tag 构建产出；日常 CI 只跑构建冒烟（mac runner `wails3 build` + 产物存在性断言），不发资产。

## 2. 资产命名（硬约束：GOOS/GOARCH 子串可被 Updater 匹配）

| 资产名 | 平台/架构 | 内容 | Updater 热更 |
|---|---|---|---|
| `cc-select-desktop_<version>_darwin_arm64.zip` | macOS arm64 | `CC Select.app/`（unsigned，A2） | ✅ zip 是 mac 热更的**唯一**合法格式 |
| `cc-select-desktop_<version>_windows_x64.zip` | Windows x64 | 安装 .exe（NSIS）+（可选）便携 .exe | ✅（updater 接受 .exe / 其 .zip） |
| `cc-select-desktop_<version>_linux_x64.AppImage` | Linux x64 | AppImage | ✅ 经 `$APPIMAGE` 原地替换 |
| `cc-select-desktop_<version>_manifest.json` | — | Updater 清单（§4） | 消费方 |
| `SHA256SUMS` | 全部 | 上述资产 + CLI 资产的 sha256（`sha256sum` 标准格式） | 校验源 |

**命名规则**：小写、下划线分隔、`_<version>_` 两侧无空格；`version` 为不带 `v` 前缀的语义化版本。GitHub provider 按 `darwin_arm64` / `windows_x64` / `linux_x64` 子串 + GOOS/GOARCH 匹配——**改名需同步 Updater 配置与测试**。

**首发明确不做**：`.dmg`（不可热更，作为「首次安装」的体验优化留待后续，若加则与 zip 并存、不进更新链）；`.msi`（不可热更）；deb/rpm（A3）；universal binary（A1 限定 M 系列）；minisign 全签名（v0.1.0 用 SHA256SUMS 摘要校验，全签名列为增强项）。

## 3. 平台分发约束（首发）

| 平台 | 首发形态 | 已知限制与文档义务 |
|---|---|---|
| macOS | unsigned .zip（内 .app） | Gatekeeper：README/Release notes 提供「右键 → 打开」指引（A2；Apple Developer 账号到位后补签名+公证，届时**同时**在 CI 加 notary 步骤并在本文档记录身份） |
| Windows | unsigned NSIS .exe（zip 内） | SmartScreen 首启提示，README 说明「更多信息 → 仍要运行」；不购买证书为当前决策 |
| Linux | AppImage | 自带 WebKitGTK 依赖；缺失场景 App 内报错指引（spec Edge Case） |

## 4. Updater 清单（`cc-select-desktop_<version>_manifest.json`）

Wails Updater 的 provider 消费结构（发布时由 CI 生成，字段以锁定的 v3.0.0-beta.34 文档为准）：

```json
{
  "version": "0.1.0",
  "products": {
    "darwin_arm64": { "path": "cc-select-desktop_0.1.0_darwin_arm64.zip", "sha256": "…" },
    "windows_x64":  { "path": "cc-select-desktop_0.1.0_windows_x64.zip",  "sha256": "…" },
    "linux_x64":    { "path": "cc-select-desktop_0.1.0_linux_x64.AppImage", "sha256": "…" }
  }
}
```

- **CurrentVersion 注入**：App 构建期经 ldflags 注入与 CLI 相同的 `internal/version.Version`（复用既有变量，单一版本真值）。
- **校验路径**：`ChecksumAsset: "SHA256SUMS"`（GitHub provider 配置）→ 下载后摘要比对，不一致按 contracts/desktop-api.md §2.2 返回 `422` 并保持当前版本完好。
- **降级保护**：只接受 `version > CurrentVersion`；同版本/低版本不触发更新提示。

## 5. App 内置 CLI（构建期嵌入契约）

- 构建顺序：`make build`（CLI）→ 拷贝 `bin/cc-select-<os>-<arch>` 至 `desktop/assets/cli/` → `wails3 build`（go:embed 嵌入）。
- **同 GOOS/GOARCH 匹配**由构建矩阵保证；嵌入文件在 desktop 构建中校验存在性（Makefile 依赖链，缺失即失败）。
- `embeddedCliVersion` 由同一 VERSION 变量注入（App 与内置 CLI 永远同版本，data-model §1.2）。

## 6. 验收断言（CI 可执行）

1. tag 构建产物名逐一匹配 §2 正则；SHA256SUMS 行数 = 资产数。
2. manifest 的每个 path 在同 Release 中存在且 sha256 一致。
3. 资产体积门（SC-004，v0.1.1 实测修订）：zip 类产物 ≤40MB；AppImage ≤100MB。
   依据：桌面二进制本体 25MB（darwin zip 13.9MB / windows zip 28.4MB 为对照）；
   AppImage 89MB = 二进制 + wails3 打包器随附的 WebKitGTK/GTK4 运行时
   （WebKitWebProcess/NetworkProcess/GPUProcess 等，保证未装 webkit 的系统可运行，
   见 wails v3 internal/commands/appimage.go）——为可移植性设计行为，非冗余。
4. macOS zip 解包顶层为 `.app`；AppImage 可执行位存在。
