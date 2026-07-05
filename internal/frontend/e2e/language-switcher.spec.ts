import { test, expect } from "./fixtures";

// 端到端覆盖语言切换：默认跟随浏览器，切换按钮可切到另一种语言，且刷新后保持。

test("语言切换按钮改变页面显示语言", async ({ page, server }) => {
  await page.goto(server.baseURL);

  // 默认情况下测试浏览器是英文环境，先验证英文文案。
  await expect(page.getByRole("heading", { name: "cc-select Configuration" })).toBeVisible();
  await expect(page.getByTestId("add-provider-button")).toHaveText("+ Add provider");

  // 切换到简体中文。
  await page.getByRole("button", { name: "简体中文" }).click();

  // 标题、按钮、提示文案变为中文。
  await expect(page.getByRole("heading", { name: "cc-select 配置" })).toBeVisible();
  await expect(page.getByTestId("add-provider-button")).toHaveText("+ 添加 provider");
  await expect(page.getByText("管理各 AI 服务商配置")).toBeVisible();

  // 刷新后保持中文（localStorage 持久化）。
  await page.reload();
  await expect(page.getByRole("heading", { name: "cc-select 配置" })).toBeVisible();
  await expect(page.getByRole("button", { name: "简体中文" })).toHaveClass(/active/);
});
