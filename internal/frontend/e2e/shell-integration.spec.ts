import { test, expect } from "./fixtures";
import { readFile } from "fs/promises";
import path from "path";

// 端到端覆盖 shell 集成一键安装的真实浏览器流程。
// isolatedServer fixture 隔离 home（HOME/USERPROFILE → 临时目录）+ 固定 zsh，
// 故 banner 检测/写入的 rc 落在 isolatedServer.homeDir，无副作用。

test("未装时显示 banner，一键安装写入 ~/.zshrc（含 marker 与 ccs）", async ({ page, isolatedServer }) => {
  await page.goto(isolatedServer.baseURL);

  // 未装：banner 显示「尚未安装」+ 一键安装按钮。
  await expect(page.getByText(/尚未安装 shell 集成/)).toBeVisible();
  const installBtn = page.getByRole("button", { name: "一键安装" });
  await expect(installBtn).toBeVisible();

  // 点击安装。
  await installBtn.click();

  // done：显示 ✅ 成功提示。
  await expect(page.getByText(/✅/)).toBeVisible();

  // 磁盘：临时 home 的 .zshrc 被写入，含 marker 块与 ccs() 定义（真实落盘）。
  const body = await readFile(path.join(isolatedServer.homeDir, ".zshrc"), "utf8");
  expect(body).toContain("cc-select shell integration");
  expect(body).toContain("ccs()");
});

test("安装后刷新页面不再显示 banner", async ({ page, isolatedServer }) => {
  // 先经 API 装好（绕过 UI），再打开页面验证 banner 隐藏。
  const res = await page.request.post(`${isolatedServer.baseURL}/api/v1/shell-integration/install`, {
    data: { shell: "zsh" },
  });
  expect(res.ok()).toBeTruthy();

  await page.goto(isolatedServer.baseURL);
  // 已装：一键安装按钮不应出现。
  await expect(page.getByRole("button", { name: "一键安装" })).toHaveCount(0);
});
