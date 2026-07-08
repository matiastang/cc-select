import { test, expect } from "./fixtures";

// 端到端覆盖本次改造的 4 个 UI 行为（后端逻辑已由 internal/web/api_test.go 覆盖，
// 这里专测浏览器里真实的交互——单测覆盖不到的部分）。

test("添加 provider：通过 preset 下拉自动填充默认配置", async ({ page, server }) => {
  await page.goto(server.baseURL);
  await page.getByTestId("add-provider-button").click();

  // 新设计：表单和 JSON 在同一视图。
  await expect(page.getByTestId("provider-json-textarea")).toBeVisible();
  await expect(page.getByTestId("preset-select")).toBeVisible();
  await expect(page.getByTestId("env-field-ANTHROPIC_BASE_URL")).toBeVisible();

  await page.getByTestId("provider-id-input").fill("e2e-preset");
  await page.getByTestId("preset-select").selectOption("deepseek");

  // 选择 preset 后，baseURL、model、JSON 都应同步更新。
  await expect(page.getByTestId("env-field-ANTHROPIC_BASE_URL")).toHaveValue(
    "https://api.deepseek.com/anthropic",
  );
  await expect(page.getByTestId("env-field-ANTHROPIC_MODEL")).toHaveValue("deepseek-v4-pro");
  await expect(page.getByTestId("provider-json-textarea")).toContainText(
    "https://api.deepseek.com/anthropic",
  );

  // 填写 API key 后保存。
  await page.getByTestId("provider-api-key-input").fill("sk-e2e");
  await page.getByTestId("provider-save-button").click();

  // 列表出现新 provider。
  await expect(page.getByTestId("provider-card-e2e-preset")).toBeVisible();

  // 经 API 确认磁盘 settings.json 含 DeepSeek 默认配置和 API key。
  const res = await page.request.get(`${server.baseURL}/api/v1/providers/e2e-preset`);
  const body = await res.json();
  expect(body.settings.env.ANTHROPIC_BASE_URL).toBe("https://api.deepseek.com/anthropic");
  expect(body.settings.env.ANTHROPIC_MODEL).toBe("deepseek-v4-pro");
  expect(body.settings.env.ANTHROPIC_AUTH_TOKEN).toBe("sk-e2e");
});

test("表单和 JSON 双向同步", async ({ page, server }) => {
  await page.goto(server.baseURL);
  await page.getByTestId("add-provider-button").click();
  await page.getByTestId("provider-id-input").fill("e2e-sync");

  // 表单 -> JSON：修改 baseURL 后 JSON 应同步。
  await page.getByTestId("env-field-ANTHROPIC_BASE_URL").fill("https://form-sync.example.com");
  await expect(page.getByTestId("provider-json-textarea")).toContainText(
    "https://form-sync.example.com",
  );

  // JSON -> 表单：修改 JSON 后表单应同步。
  await page.getByTestId("provider-json-textarea").fill(
    JSON.stringify({ env: { ANTHROPIC_BASE_URL: "https://json-sync.example.com" } }, null, 2),
  );
  await expect(page.getByTestId("env-field-ANTHROPIC_BASE_URL")).toHaveValue(
    "https://json-sync.example.com",
  );

  // API key 为必填，需填写后才能保存。
  await page.getByTestId("provider-api-key-input").fill("sk-sync");

  // 保存后落盘的是最终 JSON。
  await page.getByTestId("provider-save-button").click();
  await expect(page.getByTestId("provider-card-e2e-sync")).toBeVisible();

  const res = await page.request.get(`${server.baseURL}/api/v1/providers/e2e-sync`);
  const body = await res.json();
  expect(body.settings.env.ANTHROPIC_BASE_URL).toBe("https://json-sync.example.com");
  expect(body.settings.env.ANTHROPIC_AUTH_TOKEN).toBe("sk-sync");
});

test("编辑已有 provider：表单回填磁盘内容并可保存", async ({ page, server }) => {
  // 先经 API 建一个 provider（带 API key，否则保存时前端校验不通过）。
  await page.request.post(`${server.baseURL}/api/v1/providers`, {
    data: {
      id: "e2e-edit",
      settings: {
        env: {
          ANTHROPIC_BASE_URL: "https://orig.example.com",
          ANTHROPIC_MODEL: "orig-model",
          ANTHROPIC_AUTH_TOKEN: "sk-edit",
        },
      },
    },
  });

  await page.goto(server.baseURL);
  await page.getByTestId("edit-provider-e2e-edit").click();

  // 编辑页应回填已有值。
  await expect(page.getByTestId("env-field-ANTHROPIC_BASE_URL")).toHaveValue(
    "https://orig.example.com",
  );
  await expect(page.getByTestId("provider-json-textarea")).toContainText("orig-model");

  // 修改 model 并保存。
  await page.getByTestId("env-field-ANTHROPIC_MODEL").fill("edited-model");
  await page.getByTestId("provider-save-button").click();

  const res = await page.request.get(`${server.baseURL}/api/v1/providers/e2e-edit`);
  const body = await res.json();
  expect(body.settings.env.ANTHROPIC_MODEL).toBe("edited-model");
});

test("full 模式下非 env 字段随 JSON 一起落盘", async ({ page, server }) => {
  await page.goto(server.baseURL);
  await page.getByTestId("add-provider-button").click();
  await page.getByTestId("provider-id-input").fill("e2e-full");
  await page.getByTestId("provider-mode-select").selectOption("full");

  // 在 JSON 里加入 env 之外的字段，并保留必填的 API key。
  await page.getByTestId("provider-json-textarea").fill(
    JSON.stringify(
      { env: { ANTHROPIC_BASE_URL: "https://e2e-full", ANTHROPIC_AUTH_TOKEN: "sk-full" }, model: "opusplan" },
      null,
      2,
    ),
  );
  await page.getByTestId("provider-save-button").click();

  await expect(page.getByTestId("provider-card-e2e-full")).toBeVisible();

  const res = await page.request.get(`${server.baseURL}/api/v1/providers/e2e-full`);
  const body = await res.json();
  expect(body.settings.model).toBe("opusplan");
  expect(body.settings.env.ANTHROPIC_BASE_URL).toBe("https://e2e-full");
  expect(body.settings.env.ANTHROPIC_AUTH_TOKEN).toBe("sk-full");
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
