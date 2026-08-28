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
  // 迁移结果提示出现（本环境 provider 无明文密钥时 migrated=0，提示仍应渲染）。
  await expect(page.getByTestId("mode-migrated-notice")).toBeVisible();

  // 刷新后偏好持久（notice 是瞬态，刷新后消失属预期）。
  await page.reload();
  await expect(page.getByTestId("global-mode-select")).toHaveValue("proxy");
  await expect(page.getByTestId("mode-migrated-notice")).toHaveCount(0);
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
