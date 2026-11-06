# 分发与安装

> 本文回答需求 [R5](./requirements.md#r5-安装体验对标现有工具)：用户怎么一键装上、怎么用上 `ccs` 命令和配置页。
> 上游：GUI 已定 **双形态并存**（本地 Web + Wails v3 桌面 App，[Q2](./requirements.md#待用户确认的开放问题含已定决策)），OS 跨三平台（[Q6](./requirements.md#待用户确认的开放问题含已定决策)）。桌面 App 的包管理器分发（Cask/winget）作为后续可选渠道（见 §4）。

---

## 1. 选定方案：CLI（含 Web 配置页）+ 可选桌面 App

GUI 为双形态：本地 Web 服务（`cc-select gui`）内置于 CLI，开箱即用；Wails v3 桌面 App（v0.1.0 起，Issue #12）作为**可选**增强从 GitHub Releases 单独下载（见 ⑤）。不装桌面 App 也功能完整——一条命令装好，一条命令开配置页；装了则双击即开、可托盘常驻。

安装分两步（三平台通用思路；桌面 App 为可选增强，见 ⑤）：

### ① 装 CLI（含 gui 能力）

项目使用 **Go** 实现（[Q1](./requirements.md#待用户确认的开放问题含已定决策) 已定），通过 GitHub Releases 发布跨平台单二进制：

| 平台 | 推荐方式 | 备选方式 |
|---|---|---|
| macOS / Linux | `brew tap matiastang/cc-select && brew install cc-select` | `curl -fsSL https://raw.githubusercontent.com/matiastang/cc-select/main/scripts/install.sh \| sh` |
| Windows | `scoop bucket add cc-select https://github.com/matiastang/scoop-cc-select && scoop install cc-select` | `irm https://raw.githubusercontent.com/matiastang/cc-select/main/scripts/install.ps1 \| iex` |
| 通用 | 下载 Release 二进制放入 PATH；或从源码构建 `make all` → `./bin/cc-select` | — |

官方安装脚本同样支持**更新**：重新执行即可检测已有安装目录并替换其中的二进制。脚本从 GitHub Releases 拉取对应平台的 archive，校验 SHA-256，备份旧二进制，然后安装到 `~/.local/bin`（Unix）或 `%LOCALAPPDATA%\cc-select`（Windows）。

Release 构建由 `.goreleaser.yaml` + GitHub Actions（见 `.github/workflows/release.yml`）自动完成，产物覆盖 darwin/linux 的 amd64/arm64，以及 windows 的 amd64（windows/arm64 当前被排除）。

### ② 注入 shell 集成（首次）

```bash
cc-select init >> ~/.zshrc && source ~/.zshrc
```

`init` 按当前 shell 类型生成 `ccs` 函数（zsh/bash/PowerShell，见 [tech-stack §5](./tech-stack.md#5-跨平台与-shellq5q6)）。

### ③ 用

```bash
ccs use glm          # 切换
cc-select gui        # 开浏览器配置页
```

### ④ 包管理器发布机制

- **Homebrew Tap**：`matiastang/homebrew-cc-select`，Formula 路径 `Formula/cc-select.rb`。
- **Scoop Bucket**：`matiastang/scoop-cc-select`，manifest 路径 `cc-select.json`。

两个仓库的 manifest 均由 `.goreleaser.yaml` 中的 `brews` / `scoops` 发布器在 Release 工作流中**自动更新**：GoReleaser 在发布 GitHub Release 后，自动计算各平台 archive 的 SHA-256，提交并推送更新后的 Formula / manifest 到对应仓库。日常无需手动维护。

> 注意：即使通过包管理器安装，首次仍需执行 `cc-select init`（或打开 `cc-select gui` 一键安装）注入 `ccs` shell 函数。

### ⑤ 桌面 App（可选，v0.1.0 起）

桌面 App 与 CLI 发布在**同一个 GitHub Release**，产物以 `cc-select-desktop_` 前缀区分（命名契约见 [specs/003 contracts/release-assets](../specs/003-desktop-client/contracts/release-assets.md)）：

| 平台 | 产物 | 首启提示 |
|---|---|---|
| macOS（Apple Silicon） | `cc-select-desktop_<v>_darwin_arm64.zip`（内含 unsigned `.app`） | Gatekeeper：右键 → 打开 |
| Windows x64 | `cc-select-desktop_<v>_windows_x64.zip`（NSIS 安装器） | SmartScreen：更多信息 → 仍要运行 |
| Linux x64 | `cc-select-desktop_<v>_linux_x64.AppImage` | 依赖系统 WebKitGTK，缺失时应用内报错指引 |

- 桌面 App 与 CLI / Web 共享同一份 `~/.cc-select/` 数据与语言偏好；支持托盘常驻、开机自启、应用内一键安装 `ccs` 到 PATH、应用内自更新（同一 Releases 通道）。
- macOS 桌面端当前仅出 arm64 包（specs/003 决策 A1，限定 Apple Silicon）；CLI 侧 darwin amd64/arm64 均有。
- 使用指引见 [README - Desktop app](../README.md#desktop-app)（en/zh/ja 同步）。

---

## 2. Web 配置页一键安装 shell 集成（已实现）

> 实现：`internal/rcinteg`（引擎/策略）+ `internal/web`（API）+ 前端 `ShellIntegrationBanner`。

首次访问 Web 配置页（`cc-select gui`）时，后端检测当前 shell 的 rc 文件是否已注入 `ccs`；未注入则页面顶部弹 banner「检测到尚未安装 shell 集成」+ **[一键安装]**，点击即由后端代为写入对应 rc 文件。

### 平台支持矩阵

| 平台 | shell | 自动写入 rc | 体验 |
|---|---|---|---|
| macOS | zsh | ✅ `~/.zshrc` | 真·一键 |
| Linux | bash / zsh | ✅ `~/.bashrc`（或 `.bash_profile`）/ `~/.zshrc` | 真·一键 |
| Windows | PowerShell | ⚠️ best-effort `$PROFILE` | 见降级阶梯 |

### 关键设计

- **marker 块**：rc 中受管段带固定 begin/end 标记，使检测 / 幂等（点多次只写一次）/ 升级（snippet 变化整块替换）/ 未来卸载共用同一机制。首次写入前自动备份 `<rc>.cc-select.bak`（不覆盖已有备份）。
- **CLI/Web 同源**：`cc-select init` 与 Web 安装共用 `rcinteg.RenderInit`，snippet 永不漂移。
- **扩展点 = shell 非 OS**：加 shell 只加一个 `Strategy`，不改控制流。
- **PowerShell 委托探测**：不硬算 `$PROFILE`（PS5/PS7、OneDrive 重定向、跨平台 PS Core 路径各异），而是跑 `pwsh -NoProfile -Command '$PROFILE'`（Windows 再回退 `powershell.exe`）让 PS 自报真实路径。

### Windows 降级阶梯

1. 探测到 PowerShell 且 `$PROFILE` 可写 → 自动写入（`appended`）。
2. 探测到 PowerShell 但写失败（权限/只读）→ 返回 snippet + 命令（`manual`），前端「复制并手动执行」。
3. 完全没有 PowerShell（罕见）→ `supported:false`，前端提示。

> **WSL 限制**：Windows 原生 server（在 PowerShell/cmd 里跑 `cc-select gui`）访问不到 WSL 内的 `\\wsl$\...\.bashrc`。WSL 用户需在其 WSL 终端里跑 `cc-select init >> ~/.bashrc`。
> **fish**：不支持（项目既定），banner 显示「fish 暂不支持」。

### API

- `GET /api/v1/shell-integration` → `{supported, shell, installed, legacy, rcPath, canAutoInstall}`
- `POST /api/v1/shell-integration/install` → `{action: appended|updated|noop|manual, shell, rcPath, snippet, message}`

---

## 3. 自更新（已实现）

`cc-select update` 命令与 Web 配置页 Header 的更新按钮共用 `internal/updater` 内核
（CLI/Web 同源，同 `rcinteg` 模式），从 GitHub Releases 原地更新二进制。
桌面 App 的自更新走另一条链路：`desktop/updatebridge.go` 将 update 端点桥接到
Wails Updater，消费同一 Release 的 `cc-select-desktop_*` 资产与 manifest
（macOS zip / Windows exe / Linux AppImage 原地替换）；CLI 更新器不管理桌面 App，反之亦然。

### 用法

```bash
cc-select update              # 检查 → 下载 → SHA-256 校验 → 原子替换
cc-select update --check      # 仅检查，不安装
cc-select update --dry-run    # 下载+校验但不替换
cc-select update --force      # 即使已最新也重装（修复损坏二进制）
cc-select update --prerelease # 含预发布版本
```

Web：`GET /api/v1/update/check` → `{currentVersion, latestVersion, hasUpdate, devBuild, assetName, releaseNotes, htmlUrl}`；
`POST /api/v1/update` → `{status, fromVersion, toVersion, restartRequired}`，
策略拒绝时返回 `409 {refused:true, kind: dev|homebrew|scoop|notWritable}`。

### 关键行为

- **拒绝自更新的场景**：dev 构建（`version=dev`，请用安装脚本/`make install`）；
  Homebrew 安装（路径含 `/Cellar/`，请 `brew upgrade cc-select`）；
  Scoop 安装（路径含 `\scoop\apps\`，请 `scoop update cc-select`）；
  安装目录不可写（提示 sudo/管理员）。拒绝不是失败（CLI exit 0）。
- **替换语义**：Unix 先备份 `<exe>.bak` 再 `rename` 原子替换（运行中进程持有旧 inode）；
  Windows 走 `.old` rename dance（运行中的 exe 先改名再移入新文件，移植自 `install.ps1`）。
  **已在运行的进程（含 `cc-select gui` 本身）仍是旧版本，重启后生效**。
- **并发**：单进程内由 Web `updateMu` 互斥；跨进程由 `~/.cc-select/update.lock`
  （mtime 超过 10 分钟视为陈旧可窃取）防护。
- **限流**：GitHub API 未认证 60/hr；设置 `GITHUB_TOKEN`/`GH_TOKEN` 环境变量升至 5000/hr。

### 信任边界

Release 未做代码签名。更新器经 **HTTPS + SHA-256**（对同一 release 的 `checksums.txt`）
校验下载，与 `scripts/install.sh` / `install.ps1` 等同。开启 Smart App Control 的
Windows 上，替换后的二进制与全新安装一样可能被拦截（见
[windows-support §7](./windows-support.md#7-smart-app-control-与未签名可执行文件)）。
未来如需更强保障，可在 `updater` 的校验接缝处引入 cosign/sigstore，无需改架构。

---

## 4. 桌面 App 的包管理器分发（后续可选）

桌面 App（Wails v3）已随 GitHub Releases 直接分发（见 §1⑤），当前**无需包管理器**即可安装。一些现有全局切换工具（如 cc-switch）额外走 App Store 式的包管理器收录（Homebrew Cask / winget），该渠道作为**后续可选增强**保留。原理：`brew install --cask <name>` 让 Homebrew 自动「下载 `.dmg` → 挂载 → 拖进 `/Applications`」。

| Homebrew 类型 | 装什么 | 装到哪 |
|---|---|---|
| `brew install <formula>` | 命令行程序 | `/opt/homebrew/bin` |
| `brew install --cask <cask>` | **GUI 桌面 App**（.dmg/.app） | `/Applications` |

若日后收录，则：

```bash
brew install --cask cc-select        # macOS
winget install cc-select             # Windows
```

> 这套机制与具体 GUI 框架无关——能产出 `.dmg/.app`（macOS）、`.msi`（Windows）、`.deb/.rpm`（Linux）即可（当前首发产物为 zip / AppImage，`.dmg`/`.msi`/deb/rpm 见 specs/003 契约「首发明确不做」）。Cask formula 需在发布首个带安装包的 Release 后向 [homebrew-cask](https://github.com/Homebrew/homebrew-cask) 提 PR 收录。

---

## 5. 跨平台注意（Q6）

- **macOS/Linux**：`export`/`unset` + `.zshrc`/`.bashrc`，机制一致。
- **Windows**：PowerShell 集成已实现，含 `$PROFILE` 委托探测、UTF-8 BOM 写入与加载验证，由 CI 的「Windows PowerShell integration」步骤覆盖。CMD 不支持（见 [windows-support §4](./windows-support.md#4-为何不支持-cmd)）。

---

## 6. 维护者参考

包管理器 manifest 的自动发布、Token 配置、发版验证等维护者流程，详见 [docs/release.md](./release.md)。
