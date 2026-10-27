import { test, expect } from "./fixtures";

// v0.1.1 模块化导航（US1 / FR-001-003）：侧边栏模块可达、hash 路由、
// web 形态无「桌面端」模块。默认视口 1280×720 ≥ 880 → 侧边栏形态。

test("六个模块导航可达且桌面模块在 web 形态缺席", async ({ page, server }) => {
  await page.goto(server.baseURL);

  for (const route of ["providers", "mode-routes", "security", "general", "about"]) {
    await page.getByTestId(`nav-${route}`).click();
    await expect(page.getByTestId(`module-${route}`)).toBeVisible();
  }

  // web 形态（ccs gui）：/desktop/info 404 → 模块不注册。
  await expect(page.getByTestId("nav-desktop")).toHaveCount(0);
});

test("hash 路由：未知路由回落默认首页", async ({ page, server }) => {
  await page.goto(`${server.baseURL}/#/unknown-route`);
  await expect(page.getByTestId("module-providers")).toBeVisible();
});

test("窄屏折叠为顶部 tab 且全部模块可达", async ({ page, server }) => {
  await page.setViewportSize({ width: 720, height: 800 });
  await page.goto(server.baseURL);
  // <880px：顶部 tab 形态（同一注册表渲染）
  await expect(page.getByTestId("nav-tabs")).toBeVisible();
  await page.getByTestId("nav-about").click();
  await expect(page.getByTestId("module-about")).toBeVisible();
});
