import { test, expect } from "./fixtures";

// v0.1.1 UI 增强（US2/US3/US6/US7 的确定性子集——不触外网的部分）：
// 画廊来源徽标、向导基础步 probe 按钮禁用态、用量面板存在性、空状态。
// probe/usage 的网络行为由 Go 单测（httptest）与人工验收清单覆盖。

test("预设画廊含来源徽标与分类分组", async ({ page, server }) => {
  await page.goto(server.baseURL);
  await page.getByTestId("add-provider-button").click();
  await expect(page.getByTestId("preset-gallery")).toBeVisible();
  await expect(page.getByTestId("presets-source-badge")).toBeVisible();
  // v0.1.1 新预设（FR-010）：MiniMax 与通义千问在列。
  await expect(page.getByTestId("preset-gallery-item-minimax")).toBeVisible();
  await expect(page.getByTestId("preset-gallery-item-qwen-coding")).toBeVisible();
  await page.keyboard.press("Escape");
});

test("向导基础步：probe 按钮在未填 key 时禁用", async ({ page, server }) => {
  await page.goto(server.baseURL);
  await page.getByTestId("add-provider-button").click();
  await page.getByTestId("preset-gallery-item-deepseek").click();

  const probe = page.getByTestId("probe-test-button");
  await expect(probe).toBeVisible();
  // 模板已带 base URL+模型，但 key 未填 → 禁用。
  await expect(probe).toBeDisabled();

  await page.getByTestId("provider-api-key-input").fill("sk-e2e");
  await expect(probe).toBeEnabled();
});

test("provider 卡片内嵌用量面板（手动刷新控件存在）", async ({ page, server }) => {
  await page.goto(server.baseURL);
  await page.getByTestId("add-provider-button").click();
  await page.getByTestId("preset-gallery-item-deepseek").click();
  await page.getByTestId("provider-id-input").fill("e2e-usage");
  await page.getByTestId("provider-api-key-input").fill("sk-e2e");
  await page.getByTestId("provider-save-button").click();
  await expect(page.getByTestId("provider-card-e2e-usage")).toBeVisible();

  const panel = page.getByTestId("provider-card-e2e-usage").getByTestId("usage-panel");
  await expect(panel).toBeVisible();
  await expect(panel.getByTestId("usage-refresh-button")).toBeVisible();
});

test("仅有官方默认 provider 时显示空状态引导", async ({ page, server }) => {
  // 首启播种的官方 provider 内置不可删——「仅有官方」即空状态的实际可达形态。
  await page.goto(server.baseURL);
  const empty = page.getByTestId("providers-empty-state");
  await expect(empty).toBeVisible();
  // 空状态 CTA 打开画廊。
  await empty.getByRole("button").click();
  await expect(page.getByTestId("preset-gallery")).toBeVisible();
});
