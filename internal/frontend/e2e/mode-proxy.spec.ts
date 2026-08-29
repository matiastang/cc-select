import { test, expect } from "./fixtures";

// 端到端覆盖 Mode P（proxy）：全局模式选择器含 proxy 选项、可切换保存；
// 活跃路由面板渲染 daemon 状态与路由表，ensure 按钮可点。

test("全局模式选择器含 proxy 选项并保存成功", async ({ page, server }) => {
  await page.goto(server.baseURL);

  const select = page.getByTestId("global-mode-select");
  await expect(select).toBeVisible();
  // 值域含 proxy。
  const option = select.locator('option[value="proxy"]');
  await expect(option).toHaveCount(1);

  // 切到 proxy → 保存 → 回显。
  await select.selectOption("proxy");
  await expect(select).toHaveValue("proxy");
  // 刷新后偏好持久。
  await page.reload();
  await expect(page.getByTestId("global-mode-select")).toHaveValue("proxy");
});

test("活跃路由面板渲染 daemon 状态与路由表", async ({ page, server }) => {
  await page.goto(server.baseURL);

  const panel = page.getByTestId("routes-panel");
  await expect(panel).toBeVisible();
  // daemon 状态徽标二选一。
  await expect(panel.getByTestId("router-badge")).toHaveText(/router (running|down)/);
  // 空表也有表头渲染。
  await expect(panel.getByText("TID")).toBeVisible();
  // ensure 按钮存在且可点（不因面板异常而消失）。
  await expect(panel.getByTestId("router-ensure-button")).toBeVisible();
});

test("钥匙串开关默认关闭，显式开启后偏好持久", async ({ page, server }) => {
  await page.goto(server.baseURL);
  const toggle = page.getByTestId("keychain-toggle");
  await expect(toggle).toBeVisible();
  await expect(toggle).not.toBeChecked(); // 默认关闭：明文原样

  await toggle.click();
  await expect(toggle).toBeChecked();
  await page.reload();
  await expect(page.getByTestId("keychain-toggle")).toBeChecked(); // 偏好持久
});
