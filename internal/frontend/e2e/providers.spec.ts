import { test, expect } from "./fixtures";
import { readFile, writeFile } from "fs/promises";
import path from "path";

// 端到端覆盖本次改造的 4 个 UI 行为（后端逻辑已由 internal/web/api_test.go 覆盖，
// 这里专测浏览器里真实的交互——单测覆盖不到的部分）。

test("添加 provider：只有 JSON 表单，完整 settings（含非 env 字段）落盘", async ({ page, server }) => {
  await page.goto(server.baseURL);
  await page.getByTestId("add-provider-button").click();

  // 只支持 JSON：能看到 settings.json 编辑区，看不到旧的逐字段输入。
  await expect(page.getByTestId("provider-json-textarea")).toBeVisible();
  await expect(page.getByTestId("provider-id-input")).toBeVisible();
  await expect(page.locator("input[placeholder='ANTHROPIC_MODEL（可留空）']")).toHaveCount(0);

  await page.getByTestId("provider-id-input").fill("e2e-add");
  // 完整 settings：env 之外还带 model（非 env 字段）。
  await page.getByTestId("provider-json-textarea").fill(
    JSON.stringify({ env: { ANTHROPIC_BASE_URL: "https://e2e-add" }, model: "opusplan" }, null, 2),
  );
  // 选 full 模式：默认 settings-only 只持久化 env，会让非 env 字段（model）丢失。
  await page.getByTestId("provider-mode-select").selectOption("full");
  await page.getByTestId("provider-save-button").click();

  // 列表出现新 provider。
  await expect(page.getByTestId("provider-card-e2e-add")).toBeVisible();

  // 经 API 确认磁盘 settings.json 含非 env 字段 model（完整 settings 真落盘）。
  const res = await page.request.get(`${server.baseURL}/api/v1/providers/e2e-add`);
  const body = await res.json();
  expect(body.settings.model).toBe("opusplan");
  expect(body.settings.env.ANTHROPIC_BASE_URL).toBe("https://e2e-add");
});

test("编辑：textarea 反映磁盘真实内容（含手动改的文件）", async ({ page, server }) => {
  // 先经 API 建一个 provider（生成 profile 目录）。
  await page.request.post(`${server.baseURL}/api/v1/providers`, {
    data: { id: "e2e-edit", settings: { env: { ANTHROPIC_BASE_URL: "https://orig" } } },
  });

  // 绕过 web，直接手改磁盘 settings.json：加一个 model 字段。
  const file = path.join(server.configDir, "profiles", "e2e-edit", "settings.json");
  const cur = JSON.parse(await readFile(file, "utf8"));
  cur.model = "sonnet-MANUAL";
  await writeFile(file, JSON.stringify(cur, null, 2));

  // 打开页面点编辑：textarea 应现读磁盘真值，含手改的 sonnet-MANUAL。
  await page.goto(server.baseURL);
  await page.getByTestId("edit-provider-e2e-edit").click();
  await expect(page.getByTestId("provider-json-textarea")).toHaveValue(/sonnet-MANUAL/);
});

test("官方 provider：不渲染编辑/删除按钮，显示专属文案", async ({ page, server }) => {
  await page.goto(server.baseURL);
  const card = page.getByTestId("provider-card-claude-official");
  await expect(card).toBeVisible();
  // 官方 provider 无可编辑 settings，前端直接不渲染编辑/删除按钮（后端 PUT/DELETE 同样拒绝）。
  await expect(card.getByTestId(/^edit-provider-/)).toHaveCount(0);
  await expect(card.getByTestId(/^delete-provider-/)).toHaveCount(0);
  await expect(card).toContainText("Uses system default config");
});

test("添加：非法 JSON 被前端拦截，不会创建 provider", async ({ page, server }) => {
  await page.goto(server.baseURL);
  await page.getByTestId("add-provider-button").click();
  await page.getByTestId("provider-id-input").fill("e2e-bad");

  // 残缺 JSON：前端解析失败并提示。
  await page.getByTestId("provider-json-textarea").fill("{ not valid json");
  await page.getByTestId("provider-save-button").click();
  await expect(page.getByTestId("provider-form-error")).toBeVisible();

  // 合法 JSON 但非对象（数组）：前端拦截并提示。
  await page.getByTestId("provider-json-textarea").fill("[1, 2, 3]");
  await page.getByTestId("provider-save-button").click();
  await expect(page.getByTestId("provider-form-error")).toBeVisible();

  // 始终没有创建成功。
  const res = await page.request.get(`${server.baseURL}/api/v1/providers/e2e-bad`);
  expect(res.status()).toBe(404);
});
