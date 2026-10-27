# Feature Specification: Wails v3 桌面客户端（desktop-client）

**Feature Branch**: `003-desktop-client`

**Created**: 2026-10-26

**Status**: Draft

**Input**: User description: "v0.1.0 / [Issue #12](https://github.com/matiastang/cc-select/issues/12)：使用 Wails v3（当前最新 beta）完成桌面端客户端——用户直接打开应用即可完成 provider 配置，无需先 `ccs gui` 再开浏览器。已确认决策（2026-10-26）：① App 内置 `ccs` 并提供『安装到 PATH』按钮；② macOS 首发不签名（文档说明右键打开），Apple Developer 账号到位后补公证；③ Linux 首发 AppImage 一种格式；④ 更新渠道沿用 GitHub Releases；⑤ 托盘常驻 + 快捷切换面板、开机自启均纳入 v0.1.0。需求真值：docs/requirements.md「### v0.1.0」"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 打开应用即完成配置（Priority: P1）

用户（尤其是从 cc-switch 迁来的用户和新用户）下载安装桌面 App 后，双击图标直接进入配置界面：增删改 provider、管理 keychain、管理 Mode P 路由、切换语言与主题——全程不需要打开终端，也不需要 `ccs gui` + 浏览器。界面能力与现有 Web GUI 等价。

**Why this priority**: 这是 Issue #12 的直接动机——现有形态要求「先有终端 → 起服务 → 开浏览器」，路径长、依赖命令行；桌面 App 的核心价值就是消除这条路径。独立交付即完整。

**Independent Test**: 全新环境安装 App → 双击打开 → 完成 provider 增改删 → 在终端用 `ccs use <id>` 验证配置真实生效（同一份数据）。

**Acceptance Scenarios**:

1. **Given** 已安装桌面 App，**When** 双击图标启动，**Then** 直接出现主窗口，无需任何终端命令或浏览器操作即可进行 provider 增删改查。
2. **Given** 在 App 内保存了 provider 配置，**When** 在终端执行 `ccs use <id>`，**Then** 切换行为与纯 CLI 使用完全一致（读写同一份 `~/.cc-select/` 数据）。
3. **Given** 用户只装 App、完全不使用 CLI，**When** 仅通过 App 完成全部配置操作，**Then** 配置管理功能全部可用。
4. **Given** 现有 Web GUI 用户，**When** 使用引入桌面端之后的版本，**Then** `ccs gui` 的行为与界面与之前一致（零回归）。

---

### User Story 2 - 一键安装内置 ccs 到 PATH（Priority: P2）

用户装好 App 后，在界面里点一下「安装 ccs 到 PATH」，App 将内置的 CLI 二进制安装到当前平台的 PATH 位置；之后任意新终端可直接使用 `ccs`。重复安装能正确识别已安装版本并幂等处理。

**Why this priority**: 对标 cc-switch「命令行装完应用就自动有了」的体验（R5 精神的桌面版延伸），同时是 GUI-only 用户向 CLI 用户转化的通道。无此能力时，桌面用户与终端工作流是割裂的。

**Independent Test**: 干净环境安装 App → 点击安装按钮 → 新开终端执行 `ccs --version` 可用且版本与 App 内置一致；再次点击不产生重复文件或损坏。

**Acceptance Scenarios**:

1. **Given** 干净环境（PATH 中无 ccs），**When** 用户点击「安装到 PATH」并确认，**Then** 新开终端 `ccs --version` 可执行，版本与 App 内置 CLI 一致。
2. **Given** PATH 中已存在旧版本 ccs，**When** 再次执行安装，**Then** 识别版本差异并提示，确认后覆盖升级，不产生冲突残留文件。
3. **Given** 目标 PATH 目录无写权限，**When** 执行安装，**Then** 给出该平台对应的解决指引，不静默失败。
4. **Given** PATH 中的 ccs 来自其他来源（用户手工安装），**When** App 检测到差异，**Then** 明确提示由用户决定是否覆盖，不静默替换。

---

### User Story 3 - 应用内检查并一键更新（Priority: P3）

App 启动时自动检查新版本（可在设置中关闭），也可手动触发检查；发现新版本后展示版本信息，用户确认即自动完成下载、校验、安装并重启——全程不需要手动下载安装包或替换文件。

**Why this priority**: 用户原话「现在虽然也有更新逻辑，但是操作比较麻烦」——CLI 自更新与 GUI 里的更新按钮体验都偏繁琐；桌面 App 的自更新是品类标配预期（cc-switch 同样具备）。

**Independent Test**: 构造「当前旧版 + 远端新版」环境：启动旧版 → 收到更新提示 → 确认 → 自动完成更新并重启为新版，用户数据无损。

**Acceptance Scenarios**:

1. **Given** 当前版本低于最新发布版本，**When** App 启动（自动检查开启）或用户手动点击「检查更新」，**Then** 提示新版本号与变更说明。
2. **Given** 用户确认更新，**When** App 下载并安装，**Then** 自动完成完整性校验、安装与重启，重启后为新版本，全部用户配置与数据无损。
3. **Given** 网络不可用或更新源不可达，**When** 触发检查更新，**Then** 温和提示失败且不影响任何本地功能。
4. **Given** 更新过程被中断（如更新中强退 App），**When** 再次启动 App，**Then** 旧版本仍可正常使用，不存在半损坏状态（更新具备原子性/可恢复性）。

---

### User Story 4 - 托盘常驻与快捷切换面板（Priority: P4）

关闭主窗口后 App 可驻留系统托盘（可配置为「关闭即退出」）；托盘菜单可查看状态、快速打开主窗口，并提供 provider 快捷切换入口。

**Why this priority**: 配置工具的使用模式是「低频配置 + 偶发快速操作」，托盘让快速切换不必每次开窗。价值依赖 Story 1 先就位，故排 P4。

**Independent Test**: 关闭窗口后托盘图标存在且菜单可用；经托盘执行切换后，仅目标上下文生效，其他已运行终端的激活 provider 不变（R1 红线）。

**Acceptance Scenarios**:

1. **Given** App 运行中且驻留模式开启（默认），**When** 用户关闭主窗口，**Then** App 驻留托盘，托盘菜单可重新打开窗口。
2. **Given** 托盘菜单展开，**When** 用户查看，**Then** 可见 provider 列表与当前状态信息，可直接发起切换。
3. **Given** 多终端并行使用不同 provider，**When** 用户经托盘执行切换，**Then** 仅目标上下文按所选语义生效（语义见假设 A5），其他终端激活状态不变（R1）。
4. **Given** 设置为「关闭即退出」，**When** 用户关闭主窗口，**Then** 进程完全退出，托盘图标消失。

---

### User Story 5 - 开机自启（Priority: P5）

用户可在设置中开启「登录时自动启动」（默认关闭）；开启后 App 随系统登录启动并以托盘形态驻留，不抢占焦点。

**Why this priority**: 与托盘常驻配合的常见偏好项，实现成本低、独立可测；非核心诉求，排 P5。

**Independent Test**: 开启后注销重登（或重启），App 自动以托盘形态启动；关闭后不再随登录启动。

**Acceptance Scenarios**:

1. **Given** 已开启自启，**When** 用户登录系统，**Then** App 自动启动且不抢夺焦点（托盘形态，主窗口不弹出）。
2. **Given** 已关闭自启（默认态），**When** 用户登录系统，**Then** App 不自动启动。
3. **Given** 三平台差异（macOS 登录项 / Windows 启动项 / Linux 桌面自启机制），**When** 用户切换该开关，**Then** 三平台行为一致生效。

---

### Edge Cases

- **重复启动 App**：二次启动聚焦既有窗口（单实例），不出现第二个进程或第二个托盘图标。
- **PATH 目标目录不存在 / 无写权限 / 被安全软件拦截**：给出平台对应的解决指引，不静默失败。
- **PATH 中已有其他来源的 ccs**：版本与来源提示，由用户决定覆盖与否，不静默替换他人文件。
- **App 与已安装 CLI 版本不一致**（两者各自独立更新）：共享数据的格式需向后兼容，旧组件读新数据不得崩溃，必要字段缺失时优雅降级。
- **App 运行期间终端发生配置变更**（`ccs use` / `route switch` / `cc-select add`）：App 界面能反映最新状态（读时刷新或变更通知），不展示过期数据误导操作。
- **更新中断恢复**：下载/安装阶段中断不损坏当前版本；下次启动仍可用并可重新发起更新。
- **macOS 首发未签名**：首次打开需「右键 → 打开」绕过 Gatekeeper，发布说明与 README 提供指引；后续补签名公证不改变用户数据与升级路径。
- **Linux 缺 WebKitGTK 运行库**：以 AppImage 自带依赖为主；仍缺失时给出明确报错与安装提示，不闪退。
- **离线环境**：更新检查静默失败降级，本地配置功能完全可用。
- **无托盘环境的 Linux 窗口管理器**：不崩溃，窗口形态功能保持完整（托盘能力优雅降级）。
- **keychain 访问与 CLI 并发**：App 与 CLI 同时访问 keyring/配置文件时遵守现有原子写约定，不产生写冲突损坏。

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 桌面 App MUST 提供与现有 Web GUI 等价的配置能力：provider 增删改查、keychain 管理、Mode P 路由管理、语言与主题设置。
- **FR-002**: App 与 CLI MUST 共享同一份用户数据（现有 `~/.cc-select/` 两层存储），MUST NOT 引入第二存储或数据迁移。
- **FR-003**: 现有功能 MUST 零回归：CLI 全部子命令（`cc-select`/`ccs`）与 `ccs gui` Web 模式行为不变，既有测试全部通过。
- **FR-004**: App MUST 内置 `ccs` CLI 二进制，并提供「安装到 PATH」操作；重复安装 MUST 幂等或经版本比较后明确提示。
- **FR-005**: 「安装到 PATH」MUST 在 macOS / Linux / Windows 三平台选择正确的安装目标，权限不足时 MUST 给出指引而非静默失败。
- **FR-006**: App MUST 支持启动时自动检查更新与手动检查更新，更新源为项目现有 GitHub Releases 发布渠道。
- **FR-007**: 更新 MUST 经完整性校验后原子安装，支持自动重启进入新版；失败或中断 MUST NOT 破坏当前可用版本。
- **FR-008**: App MUST 支持系统托盘常驻，并提供「关闭窗口时驻留 / 退出」设置项。
- **FR-009**: 托盘 MUST 提供 provider 列表与状态展示、快捷切换入口；任何经 App 的切换 MUST NOT 影响其他已运行终端的激活状态（R1 红线）。
- **FR-010**: App MUST 提供「登录自启」设置（默认关闭），三平台一致生效。
- **FR-011**: App MUST 单实例运行：二次启动聚焦既有窗口，不产生重复进程/托盘。
- **FR-012**: App MUST 支持 en/zh 双语界面，语言偏好与 CLI/Web GUI 共享同一份既有偏好机制。
- **FR-013**: App MUST 覆盖三平台：macOS（仅 arm64，M 系列芯片）、Windows（x64）、Linux（x64，首发 AppImage）。
- **FR-014**: 凭证处理 MUST NOT 低于现有安全基线（宪法原则 VI）：不新增明文凭证写入路径，行为向 keychain 方向收敛。
- **FR-015**: App 界面 MUST 由现有 Web 前端单一源码演进而来（Web 与桌面共用），MUST NOT 分叉出第二套前端实现。

### Key Entities

- **桌面构建产物**：三平台安装包（macOS arm64 应用包、Windows 安装包、Linux AppImage），同一条发布流水线产出。
- **更新通道条目**：版本号、平台产物、变更说明与校验信息——由现有 GitHub Releases 流程扩展而来，与 CLI 发布同渠道。
- **CLI 安装状态**：App 内置 CLI 版本、PATH 安装位置、已安装版本——支撑幂等安装与升级提示。
- **桌面偏好项**：托盘驻留开关、自启开关、自动检查更新开关——沿用现有用户偏好存储机制扩展。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 全新环境下，用户从「下载安装」到「完成第一个 provider 配置」全程无需终端命令，5 分钟内可完成。
- **SC-002**: 更新流程从「发现新版本」到「重启进入新版」不超过 3 次用户交互，无需手动下载或替换任何文件。
- **SC-003**: 引入桌面端后，既有 CLI 与 Web GUI 的全部测试（Go 单测、集成测试、Playwright e2e）100% 通过，零回归可被 CI 验证。
- **SC-004**: 三平台产物由同一条 CI 流水线产出，安装包体积保持在轻量水平（≤ 40MB 量级），冷启动无明显卡顿。
- **SC-005**: 多终端并行使用不同 provider 的场景下，App/托盘的一切操作不产生跨终端串扰（既有 R1 验收用例全部通过）。

## Assumptions

- **A1**: 桌面框架采用 **Wails v3**（≥ beta.34；官方已宣告桌面 API 稳定）。接受 Beta 阶段其余 API 的跟进成本：锁定版本、升级走显式流程。
- **A2**: macOS 首发**不签名**（暂无 Apple Developer 账号）：发布说明与 README 提供「右键 → 打开」指引；账号到位后补签名+公证，不视为架构变更。
- **A3**: Linux 首发仅 AppImage（自带 WebKitGTK 依赖）；deb/rpm 等格式留待后续版本按需增加。
- **A4**: 更新渠道沿用 **GitHub Releases**：在现有发布流程上扩展桌面端三平台产物与校验信息，不新建分发体系。
- **A5**: 托盘快捷切换的**目标语义**在 plan 阶段定案（本 spec 只锁 R1 红线）：候选方案 a) Mode P 下针对选定终端的路由切换（`routes.json` 已有真值表）；b) Mode A/B 下提供「复制 `ccs use` 命令」的辅助形态。当前倾向 a+b 并存，按模式自动呈现。
- **A6**: 开机自启默认关闭；实现采用各 OS 原生自启机制，具体技术方案在 plan 阶段确定。
- **A7**: Windows 侧视 WebView2 运行时为系统自带（Win10/11）；「Windows 不支持 CMD」的既有约束不变（宪法原则 III）。
- **A8**: 桌面端与 Web 端共用 `internal/frontend` 单一前端源码（构建目标区分），共用现有 API 数据层，避免双实现分叉。
