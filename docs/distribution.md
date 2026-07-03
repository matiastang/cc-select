# 分发与安装

> 本文回答需求 [R5](./requirements.md#r5-安装体验对标-cc-switch)：用户怎么一键装上、怎么用上 `ccs` 命令和配置页。
> 上游：GUI 已定 **Web 路线**（[Q2](./requirements.md#待用户确认的开放问题含已定决策)），OS 跨三平台（[Q6](./requirements.md#待用户确认的开放问题含已定决策)）。桌面 App + Cask 作为**备选**保留。

---

## 1. 选定方案：CLI 含 Web 配置页，无桌面 App

因 GUI 选了本地 Web 服务（`cc-select gui`），**不再需要安装桌面 App**——用户只装一个 CLI，配置页随 `cc-select gui` 命令启动。这比 cc-switch 的"装 App"更轻：一条命令装好，一条命令开配置页。

安装分两步（三平台通用思路）：

### ① 装 CLI（含 gui 能力）

项目使用 **Go** 实现（[Q1](./requirements.md#待用户确认的开放问题含已定决策) 已定），通过 GitHub Releases 发布跨平台单二进制：

| 平台 | 推荐方式 |
|---|---|
| macOS / Linux | 下载 Release 二进制并放入 `PATH`；或后续 Homebrew formula |
| Windows | 下载 Release `.zip` 并放入 `PATH`；或后续 `winget` / `scoop` |
| 通用 | 从源码构建：`make all` → `./bin/cc-select` |

Release 构建由 `.goreleaser.yaml` + GitHub Actions（见 `.github/workflows/release.yml`）自动完成，产物覆盖 darwin/linux/windows 的 amd64/arm64。

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

## 3. 备选：桌面 App + Homebrew Cask（若日后 GUI 改回桌面 App）

cc-switch 走的是这条路，作为**备选保留**。原理：`brew install --cask <name>` 让 Homebrew 自动「下载 `.dmg` → 挂载 → 拖进 `/Applications`」。

| Homebrew 类型 | 装什么 | 装到哪 |
|---|---|---|
| `brew install <formula>` | 命令行程序 | `/opt/homebrew/bin` |
| `brew install --cask <cask>` | **GUI 桌面 App**（.dmg/.app） | `/Applications` |

若日后选定桌面 App（Tauri/Electron/Wails），则：

```bash
brew install --cask cc-select        # macOS
winget install cc-select             # Windows
```

> 这套机制与具体 GUI 框架无关——能产出 `.dmg/.app`（macOS）、`.msi`（Windows）、`.deb/.rpm`（Linux）即可。Cask formula 需在发布首个带安装包的 Release 后向 [homebrew-cask](https://github.com/Homebrew/homebrew-cask) 提 PR 收录。

---

## 4. 跨平台注意（Q6）

- **macOS/Linux**：`export`/`unset` + `.zshrc`/`.bashrc`，机制一致。
- **Windows**：PowerShell 集成已实现，含 `$PROFILE` 委托探测、UTF-8 BOM 写入与加载验证，由 CI 的「Windows PowerShell integration」步骤覆盖。CMD 不支持（见 [windows-support §4](./windows-support.md#4-为何不支持-cmd)）。
