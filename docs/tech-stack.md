# 技术选型

> 本文记录选型决策：**已定项**与**备选方案**（保留，日后可调整）。
> 上游：需求 [R4](./requirements.md#r4-图形化界面配置服务商)/[R7](./requirements.md#隐含-派生需求)，开放问题 [Q1–Q6](./requirements.md#待用户确认的开放问题含已定决策)。

---

## 0. 决策总览

| 项 | 决策 | 状态 | 备选 |
|---|---|---|---|
| GUI 形态（[Q2](./requirements.md#待用户确认的开放问题含已定决策)） | **本地 Web 服务 + 浏览器** | ✅ 已定 | 桌面 App（Tauri/Electron/Wails） |
| 存储格式（[Q3](./requirements.md#待用户确认的开放问题含已定决策)） | **单个 JSON 文件**（原子写）+ key 走 Keychain | ✅ 已定 | SQLite / 多文件拆分 |
| 目标 shell（[Q5](./requirements.md#待用户确认的开放问题含已定决策)） | **zsh / bash / PowerShell**（fish 后续接入） | ✅ 已定 | fish |
| 目标 OS（[Q6](./requirements.md#待用户确认的开放问题含已定决策)） | **macOS / Linux / Windows** | ✅ 已定 | — |
| 语言（[Q1](./requirements.md#待用户确认的开放问题含已定决策)） | **Go**（单二进制、跨平台编译省心、启动快） | ✅ 已定 | Node+TS / Rust（纯脚本已排除） |

> 关键：因 GUI 选了 Web 路线，**语言与 GUI 解耦**——任何语言都能起 HTTP 服务 + 读写 JSON。故语言可独立按 CLI 开发效率决定，不被 GUI 绑架。

---

## 1. 参考事实：cc-switch 的真实技术栈

（来源：[cc-switch package.json](https://github.com/farion1231/cc-switch)）

| 层 | 技术 |
|---|---|
| 桌面框架 | **Tauri 2**（Rust 后端 + Web 前端，单二进制打包，非 Electron） |
| 前端 | React 18 + TypeScript，Vite 构建 |
| UI | Radix UI + Tailwind CSS + CodeMirror（JSON 配置编辑器） |
| 数据/状态 | TanStack Query、Tauri plugin-store、plugin-updater（自动更新） |
| 构建/测试 | pnpm + Vite + Vitest |

即：cc-switch 是一个 **Rust（Tauri）+ React** 的轻量桌面 App。

> 注：cc-switch 是桌面 App 路线，与本项目**选定 Web 路线不同**。其前端实现（React + 表单 UI）可作为 Web 配置页 UI 的参考，但 GUI 形态本身不照搬。

---

## 2. 语言选型（Q1，已定：Go）

GUI 已定 Web 路线，与语言解耦，故语言按 **MVP 关键动作**权衡：① 生成 shell 函数（zsh/PowerShell）② 起 Web 配置页 ③ 跨三平台分发 ④ 读写 JSON ⑤ eval/iex 注入。

### 2.1 针对 MVP 的对比

| 维度 | Node + TS | Rust | Go |
|---|---|---|---|
| CLI 字符串/JSON 处理 | ⭐⭐⭐ 最顺 | ⭐ 繁琐 | ⭐⭐ 还行 |
| 起 Web 配置页 | ⭐⭐⭐ 生态最强（React 等） | ⭐⭐ axum 可用 | ⭐⭐ 标准库 + embed 够用 |
| 单二进制 / 零运行时 | ❌ 装 Node 或打包 40-80MB | ✅ 小且快 | ✅ 10-30MB |
| 跨三平台交叉编译 | ⭐ 各平台分别 build | ⭐⭐ cargo-zigbuild | ⭐⭐⭐ 内置 GOOS/GOARCH 最省心 |
| 启动速度（影响 `cc-select use` 频繁调用） | ⭐ 慢（V8 boot） | ⭐⭐⭐ 瞬时 | ⭐⭐⭐ 快 |
| npm 一键安装 | ✅ 原生 | ✅ 常见（平台包） | ✅ 可行 |

> **关键认知**：Rust/Go 二进制也能通过 **npm 分发**（npm 作为跨平台二进制渠道，按 OS/arch 拉对应预编译包，Biome/Turbo 等 Rust CLI 都这么做）。所以"npm 一键安装"非 Node 独占，**不必为此选 Node**。

### 2.2 取舍与推荐

| 路线 | 最适合 | 代价 |
|---|---|---|
| **Go**（⭐ 推荐） | 单二进制（Windows 不用装 Node）+ 跨平台编译最省心 + 启动快 + Web 够用 | JSON/字符串处理不如 TS 顺（但 provider 结构简单，负担小） |
| **Node + TS** | 最快做 CLI + Web，前端生态熟 | 启动慢；要么装 Node 要么打包臃肿 |
| **Rust** | 追求极致体积/启动，或想对标 cc-switch 同栈 | CLI 开发最繁琐，性能优势在此用不上多少 |
| **Bash/Zsh 脚本** | 仅纯 CLI 验证隔离 | **不易起 Web GUI**、Windows 不适用 → 排除 |

**推荐 Go**：cc-select 的特点是 CLI 频繁调用（切换）、跨三平台、Web 配置页中等复杂度。Go 的单二进制（对 Windows 友好）、交叉编译省心、启动快最契合；Web 用标准库 `net/http` + `embed` 打包前端即可。Node 开发更快但有分发/启动代价；Rust 对这种简单 CLI 过重。

> 备选全部保留。最终选哪个由维护者拍板，但 **纯脚本路线因 Windows + Web GUI 已排除**。详见 [windows-support.md](./windows-support.md) 对语言的反作用。

### 2.3 来源

- [Rust CLI 多平台打包经验](https://ivaniscoding.github.io/posts/rustpackaging1/)
- [HN：为何 Rust CLI 也用 npm 分发](https://news.ycombinator.com/item?id=47256648)
- [Rust 交叉编译现状](https://users.rust-lang.org/t/rust-ecosystem-needs-improvement-in-the-area-of-cross-compilation/101378)

---

## 3. GUI 形态：已定 Web 服务（Q2）

**选定：本地 Web 服务 + 浏览器**。`cc-select gui` 起本地 HTTP 服务，自动开浏览器访问配置页，通过 HTTP API 读写共享 JSON。

| 方案 | 状态 | 优点 | 缺点 |
|---|---|---|---|
| **本地 Web 服务** | ✅ 选定 | 无需打包、跨平台天然、与语言解耦、UI 迭代快 | 非常驻，每次 `cc-select gui` 起服务 |
| 桌面 App（Tauri+React） | 🔁 备选 | 体验最好、可常驻托盘、对标 cc-switch | 需打包，体积较大 |
| 桌面 App（Electron） | 🔁 备选 | 生态熟 | 体积大 |
| 桌面 App（Wails/Go） | 🔁 备选 | Go 单二进制 | 与 CLI 同语言才合适 |

> 切换路径：日后若改桌面 App，因配置存储是共享 JSON（见 §4），GUI 形态可平滑替换，对 CLI 与存储零影响。

---

## 4. 存储格式：JSON 两层（元信息 + profile 真值），Q3

**当前实现**：两层 JSON，都原子写、文件 0600：
- `~/.cc-select/providers.json`：元信息索引（id/name），不含 env/token。
- `~/.cc-select/profiles/<id>/settings.json`：该 provider 的 env 真值（当前含明文 token），claude 读这个。

**keychain 占位机制**（已预留，待完全接入）：`providers.json` 的 `env` 值可写成 `$keychain:cc-select:<id>:<var>` 占位，运行时由 `internal/secrets` 从系统 Keychain 取真值。当前 CLI/Web 写入路径仍把 token 明文落 profile settings.json，未来可平滑迁移到 keychain 占位而不改文件结构。

| 方案 | 状态 | 优点 | 缺点 |
|---|---|---|---|
| **JSON 两层（元信息 + profile）** | ✅ 已定 | 多进程并发读无锁、原子写安全、claude 直接读 profile | 当前 token 明文（0600 兜底） |
| JSON + keychain 占位 | 🟡 目标 | token 不落明文 JSON | 需每次切换时从 keychain 取值，交互/失败处理更复杂 |
| SQLite | 🔁 备选 | 结构化、多表（cc-switch 同款） | 多进程锁竞争，过度设计 |
| 多文件拆分 | 🔁 备选 | 可单独分享某 provider | 全局视图需聚合 |

> 切换机制改用 `CLAUDE_CONFIG_DIR`（见 [architecture §2.0](./architecture.md#20-切换机制claude_config_dir关键)、[engineering §6](./engineering-decisions.md#6-claude-的-settingsjson-env-优先级高于-shell已用-claude_config_dir-解决)）。profile settings.json 的 env 字段是 claude 实际读取的配置来源，因此无论 token 是明文还是 keychain 占位，最终写入 profile 时都必须能被 claude 解析为真值。

---

## 5. 跨平台与 shell（Q5/Q6）

- **OS**：macOS / Linux / Windows 三平台。Web GUI 与 JSON 跨平台天然；shell 集成与 Windows 环境变量机制需注意（见 [architecture §2.1](./architecture.md#21-跨平台约束满足-q6macoslinuxwindows)）。
- **shell**：MVP 支持 **zsh/bash**（共用 emitter）与 **PowerShell**；`init` 生成的 shell 集成按 shell 类型分发函数定义，fish 后续接入。

---

## 6. 决策状态

所有主要选型已定，MVP 已实现（见 [roadmap](./roadmap.md)）：

- [x] 语言：**Go**
- [x] GUI：本地 Web 服务
- [x] 存储：JSON 两层（profile 目录），keychain 占位机制已预留
- [x] shell：zsh/bash/PowerShell（fish 后续）
- [x] OS：macOS / Linux / Windows

> Go 下的具体依赖（`cobra`、`zalando/go-keyring`、标准库 `net/http` + `embed` 等）已在实现期落地，详见 `go.mod` 与各 `internal/` 包文档。
