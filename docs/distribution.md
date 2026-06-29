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

## 2. 体验优化：把 init 集成进 Web 配置页首次访问

为让 shell 集成也"无感"：首次访问 Web 配置页时，检测到 `~/.zshrc` 缺少 `ccs`，页面提示「一键安装 shell 集成」，点击即代为写入。整体体验接近 cc-switch 的"装完即用"。

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
- **Windows**：环境变量隔离语义不同（`setx` 写注册表 vs 进程级；PowerShell profile vs `.zshrc`），shell 集成与隔离机制需单独设计（见 [architecture §2.1](./architecture.md#21-跨平台约束满足-q6macoslinuxwindows)）。Windows 支持可能在 MVP 之后单独完善。
