# Quickstart: Wails v3 桌面客户端验证指南（desktop-client）

**Date**: 2026-10-26 | **Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

> 用途：**端到端验证手册**（开发自证 / code review 依据 / 验收回填）。实现细节见 tasks.md；本文件只含可执行步骤与预期结果。每步标注对应的 spec 验收锚点。

---

## 0. 前置条件

| 项 | 要求 |
|---|---|
| Go | 1.24+（既有项目要求） |
| Wails CLI | `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34`（**与 go.mod 锁定版本一致**） |
| 平台依赖 | macOS：Xcode CLT；Linux：`libgtk-3-dev libwebkit2gtk-4.1-dev`（AppImage 打包另需 linuxdeploy）；Windows：无额外（WebView2 系统自带） |
| 既有工具链 | node/npm（前端构建）、既有 `make check` 可全绿 |

## 1. 构建与本地运行

```bash
# 前端（产物到 internal/web/assets，桌面/Web 双形态共用）
cd internal/frontend && npm install && npm run build && cd ../..

# CLI（供 desktop 嵌入）
make build VERSION=0.1.0

# 桌面端（开发模式：热重载，API 走 AssetServer Handler）
wails3 dev -d desktop        # 形态以 tasks 中的最终 task 配置为准

# 桌面端（打包本机平台产物）
make desktop-package VERSION=0.1.0
```

**预期**：`wails3 dev` 打开原生窗口，界面与 `ccs gui` 浏览器版**像素级同源**（同一前端构建产物）；providers/keychain/路由（Mode P）面板全部可用 → Story 1 / FR-001。

**自动化**（无需 GUI）：`go test ./desktop/... ./internal/...` 全绿；`make e2e`（Playwright，web 形态回归）全绿 → FR-003 / SC-003。

## 2. 逐 Story 手工验证脚本

### Story 1 打开即配置（FR-001/FR-002/FR-003）

1. 双击打包产物启动 → 主窗口直接可用，全程未开终端。
2. 窗口内新增 provider `quickstart-test`（URL/key/model 随意）→ 保存成功。
3. 终端执行 `ccs list` → 能看到 `quickstart-test`（**同一份数据**）；`ccs use quickstart-test` → 切换成功。
4. `make test && make e2e` → 全绿（既有功能零回归）。

### Story 2 安装内置 CLI（FR-004/FR-005，契约见 contracts/desktop-api.md §3.3）

1. 干净 PATH（临时改 PATH 模拟）：设置区 → 「安装到 PATH」→ 新终端 `cc-select --version` 输出 `0.1.0`。
2. 把某旧版 cc-select 放进 PATH → 按钮显示「升级」→ 升级后版本 = 0.1.0。
3. 放一个无关同名二进制 → 安装 → 出现**覆盖确认**（409 external-binary），确认后覆盖。
4. macOS 演练无权限：`sudo chown root /usr/local/bin` 场景 → 回落 `~/.bin` 并提示加 PATH（或给出指引）。

### Story 3 应用内更新（FR-006/FR-007，通道契约见 contracts/release-assets.md）

1. 本地起一个 mock GitHub Release（或用测试仓库 tag）：发布 `0.1.1` 资产 + SHA256SUMS + manifest。
2. 用 `0.1.0` 构建启动（`autoUpdateCheck=true`）→ 启动后出现更新卡片（复用现有 CheckUpdateButton UX）。
3. 点击更新 → 下载/校验/安装自动完成 → 提示重启 → 重启后「关于」显示 `0.1.1`，providers/prefs 数据完好。
4. 断网启动 → 无更新报错弹窗，本地功能全部可用（温和降级）。
5. 更新中强杀进程 → 再启动仍为 `0.1.0` 且可再次更新（原子性）。

### Story 4 托盘（FR-008/FR-009，research R5 语义）

1. 关闭主窗口 → 托盘图标存在；点开菜单见 providers 列表与状态。
2. 点击任一 provider → 剪贴板为 `ccs use <id>` + 系统通知确认。
3. **红线验证**：终端 A `ccs use glm`、终端 B `ccs use minimax`，随后在托盘点任意 provider → 两终端 `ccs current` 各自不变 → SC-005。
4. 设置区切「关闭即退出」→ 关窗后进程退出（`ps` 确认无残留）。
5. Linux 无托盘 WM（如 sway 配置下）→ App 不崩溃，窗口功能完整。

### Story 5 自启（FR-010）

1. 开启「登录自启」→ 注销重登 → App 以托盘形态驻留、**无主窗口抢焦点**。
2. 关闭 → 重登不启动。
3. 三平台各验一遍（Linux 验 XDG autostart 文件生成/移除）。

### 单实例（FR-011）

连续双击 App 两次 → 任务管理器/活动监视器只有一个进程；第二次启动聚焦既有窗口。

## 3. 三平台矩阵冒烟（发布前，对应 §2 全项 + 平台特有项）

| 平台 | 特有验证 |
|---|---|
| macOS (arm64) | 首次打开走「右键 → 打开」路径并核对 README 指引文案；zip 产物解包顶层为 .app（release-assets §6.4） |
| Windows (x64) | SmartScreen 提示与 README 说明一致；安装 CLI 后**新开** PowerShell 会话 `ccs` 可用（HKCU PATH 生效）；CMD 不在本工具支持范围（既有约束，不测） |
| Linux (x64) | AppImage 双击可跑（含 WebKitGTK 缺失时报错指引）；`$APPIMAGE` 热更演练（Story 3 在 Linux 重跑第 3 步） |

## 4. CI 断言（contracts/release-assets.md §6 的可执行化）

tag 构建（`v0.1.0` 起）：产物名逐一匹配、SHA256SUMS 完整、manifest 与资产 sha256 一致、单平台体积 ≤ 40MB → 任一失败构建红。

## 5. 完成定义（对照 spec Success Criteria）

- [ ] SC-001：§2 Story 1 走通（安装→配置 ≤5 分钟、无终端命令）
- [ ] SC-002：§2 Story 3 ≤3 次交互完成更新
- [ ] SC-003：`make check && make test && make e2e` 全绿
- [ ] SC-004：CI 断言全绿、体积达标
- [ ] SC-005：§2 Story 4 第 3 步红线验证通过
- [ ] 全部通过后：按宪法 VII 将验收结论回流 docs/acceptance-tests.md（tasks 末项）
