# Contract: 前端架构与 UI 契约（v0.1.1）

**Spec**: [spec.md](../spec.md) FR-001–FR-011, FR-021, FR-022 | research §3.1/3.4

## 1. 依赖清单（新增，全部 devDep/dep 声明于 `internal/frontend/package.json`）

| 依赖 | 用途 |
|---|---|
| `tailwindcss@^3.4` + `postcss` + `autoprefixer` | 样式引擎（v3 定版理由见 research §2.4） |
| `tailwindcss-animate` | Radix 进出场动画类 |
| `clsx` + `tailwind-merge` | `cn()` 类合并（shadcn 标配） |
| `@radix-ui/react-{dialog,tabs,switch,select,tooltip,label,dropdown-menu,separator,toast}` | 无头原语（按实际使用的组件逐个引入） |
| `lucide-react` | 图标 |

不引入：react-router（自研 hash router）、react-query、整库组件库（Mantine/AntD）。

## 2. 设计 token 契约

- 定义于 `src/style.css`（Tailwind `@layer base` 内的 CSS 变量），命名沿用既有族（`--bg/--card/--border/--text/--muted/--accent/--danger…`）并扩展 `--radius/--font` 系列；`tailwind.config` 以 `theme.extend.colors = { bg: "var(--bg)", … }` 映射，组件**只写语义类**（`bg-card text-muted`），不写裸色值。
- 深浅色：延续 `:root` / `[data-theme="dark"]` / `@media (prefers-color-scheme: dark)` 三态结构（theme.ts 机制与 localStorage 键不变，ThemeSwitcher 交互不变）。
- 断点：**880px** 单断点（与桌面窗 MinWidth 对齐）；`<880px` 侧边栏 → 顶部水平 tab 条（同一 `nav` 数据源渲染两种形态）。

## 3. 组件契约（`src/components/ui/`）

- 既有导出名**全部保留**（Button/Card/Input/Select/Textarea/FormField/Collapsible/SegmentedControl/Icon），内部以 Tailwind + Radix 重写；既有 props 签名兼容（新增可选 props 只增不改）。
- 新增：`Dialog`、`Tabs`、`Switch`、`Tooltip`、`Label`、`Badge`、`Progress`、`Spinner`、`EmptyState`、`Toast`（Radix Toast 封装 + `useToast` context）、`Stepper`（向导步骤条，自研）。
- `Icon` 保留为兼容转发层（内部映射 lucide 同名图标），新代码直接用 lucide。

## 4. 导航与模块注册（`src/navigation/`）

```ts
export interface NavModule {
  route: HashRoute;          // "#/providers" 等
  titleKey: string;          // i18n key（common 命名空间）
  icon: LucideIcon;
  render: () => JSX.Element;
  desktopOnly?: boolean;     // true 时仅 /desktop/info 探测通过后注册
}
```

- 路由解析：`location.hash` → 未匹配/为空 → 重定向 `#/providers`；`hashchange` 驱动，无历史污染（pushState 不用）。
- 模块注册表是唯一导航真值；侧边栏与顶部 tab 由同一注册表渲染。

## 5. data-testid 兼容契约（FR-008）

- 既有 40 个 testid 全保留、语义不变；e2e 因模块化需要补的导航步骤仅允许「点击 nav item（新 testid：`nav-{route}`，如 `nav-mode-routes`）」，不得改既有断言目标。
- 新增交互新增 testid 前缀约定：画廊 `preset-gallery-*`、向导 `wizard-*`、探测 `probe-*`、用量 `usage-*`、预设源 `presets-source-*`、开关 `remote-presets-toggle`。

## 6. 关键交互流契约

### 6.1 添加向导（US3，FR-009）

1. `ProviderList` 空状态 or 「添加服务商」按钮（**保留 `add-provider-button` testid**）→ 打开画廊 `Dialog`（分类分组卡片：名称/分类徽标/一句话描述；底部「自定义配置」入口，保留 JsonForm 高级模式作为 custom 预设路径）。
2. 选卡 → 向导（`Dialog` 内 `Stepper`）：**Step1 基础** = 名称（默认预设名）+ API key（`type=password` + 显隐切换，旁附「获取 API Key」外链 = preset.apiKeyURL）；**Step2 高级（默认折叠摘要，可展开）** = 隔离模式 SegmentedControl、模型覆盖（预填模板默认值）、额外 env（复用 EnvFieldEditor）、JSON 预览（沿用既有 JSON 视图切换）。
3. 保存前校验：必填缺失就地红字；`probe` 按钮在 Step1 即可用（FR-016）。
4. 保存成功 → toast + 列表刷新；失败 → toast 错误分类文案。

### 6.2 用量面板（US7，FR-020）

- ProviderCard 展开区或详情 Dialog 内：`usage-refresh-button`（手动）→ POST usage → 窗口卡片（`Progress` + 百分比 + resetAt 倒计时）或 `usage-unsupported` 占位。fetchedAt 恒显示。

## 7. i18n 契约（FR-021）

- 新增文案落位：`common.json`（导航/模块标题）、`providers.json`（画廊/向导/用量/探测）、`desktop.json` 不动、新增 `general.json`（远端预设/Shell 集成设置区）；en/zh 同 key 同步提交，漏一边 = lint 可查（既有测试模式扩展）。

## 8. 兼容性验收（FR-022）

- CSS 特性白名单：Safari 15 基线（不用 `:has`、container queries、`@property`、`color-mix()`）；Radix Select/Dialog 已考虑旧引擎。
- 桌面三平台人工验收清单（quickstart §4）：macOS WKWebView / Windows WebView2 / Linux webkit2gtk 各走查 6 模块 + 深浅色 + 向导全流程。
