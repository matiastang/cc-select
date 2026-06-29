import { defineConfig, devices } from "@playwright/test";

// 端到端测试：驱动真实浏览器，访问由真实 cc-select 二进制 serve 的页面
// （含 Go //go:embed 的前端产物）。每个用例由 e2e/fixtures.ts 起一个独立的
// 二进制进程 + 临时 CC_SELECT_CONFIG，保证彼此隔离、可并行。
//
// globalSetup 负责先构建最新前端 assets + 二进制，确保测的是当前代码。
export default defineConfig({
  testDir: "./e2e",
  globalSetup: "./e2e/global-setup.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: "list",
  use: {
    trace: "on-first-retry",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
