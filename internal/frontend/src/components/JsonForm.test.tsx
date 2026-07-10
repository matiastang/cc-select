import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, act, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import i18n from "../i18n";
import { JsonForm } from "./JsonForm";

function renderWithI18n(ui: React.ReactNode) {
  return render(<I18nextProvider i18n={i18n}>{ui}</I18nextProvider>);
}

function res(body: Record<string, unknown>, ok = true) {
  return { ok, json: () => Promise.resolve(body) };
}

const PRESET_LIST = {
  presets: [
    {
      id: "deepseek",
      displayName: "DeepSeek",
      category: "cn_official",
      apiFormat: "anthropic",
      authField: "ANTHROPIC_AUTH_TOKEN",
      requiredVars: ["ANTHROPIC_AUTH_TOKEN"],
      optionalVars: ["ANTHROPIC_MODEL"],
      oauth: false,
    },
    {
      id: "zhipu-glm",
      displayName: "智谱 GLM",
      category: "cn_official",
      apiFormat: "anthropic",
      authField: "ANTHROPIC_AUTH_TOKEN",
      requiredVars: ["ANTHROPIC_AUTH_TOKEN"],
      optionalVars: ["ANTHROPIC_MODEL"],
      oauth: false,
    },
    {
      id: "custom",
      displayName: "Custom",
      category: "custom",
      apiFormat: "anthropic",
      requiredVars: ["ANTHROPIC_AUTH_TOKEN"],
      optionalVars: [],
      oauth: false,
    },
  ],
  categories: ["official", "cn_official", "custom"],
};

const PRESET_DETAIL = {
  id: "deepseek",
  displayName: "DeepSeek",
  category: "cn_official",
  apiFormat: "anthropic",
  authField: "ANTHROPIC_AUTH_TOKEN",
  requiredVars: ["ANTHROPIC_AUTH_TOKEN"],
  optionalVars: ["ANTHROPIC_MODEL"],
  oauth: false,
  envTemplate: {
    ANTHROPIC_BASE_URL: "https://api.deepseek.com/anthropic",
    ANTHROPIC_AUTH_TOKEN: "${API_KEY}",
    ANTHROPIC_MODEL: "deepseek-v4-pro",
  },
};

const PRESET_DETAIL_GLM = {
  id: "zhipu-glm",
  displayName: "智谱 GLM",
  category: "cn_official",
  apiFormat: "anthropic",
  authField: "ANTHROPIC_AUTH_TOKEN",
  requiredVars: ["ANTHROPIC_AUTH_TOKEN"],
  optionalVars: ["ANTHROPIC_MODEL"],
  oauth: false,
  envTemplate: {
    ANTHROPIC_BASE_URL: "https://open.bigmodel.cn/api/anthropic",
    ANTHROPIC_AUTH_TOKEN: "${API_KEY}",
    ANTHROPIC_MODEL: "glm-5.1",
  },
};

describe("JsonForm", () => {
  let fetchMock: ReturnType<typeof vi.fn>;
  let fetchCalls: { url: string; init?: RequestInit }[];

  beforeEach(async () => {
    fetchCalls = [];
    fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
      fetchCalls.push({ url, init });
      if (url.includes("/presets/zhipu-glm")) return res(PRESET_DETAIL_GLM);
      if (url.includes("/presets/deepseek")) return res(PRESET_DETAIL);
      if (url.includes("/presets")) return res(PRESET_LIST);
      if (url.includes("/providers/"))
        return res({ id: "glm", name: "", settings: { env: {} }, isolationMode: "" });
      return res({});
    });
    vi.stubGlobal("fetch", fetchMock as unknown as typeof fetch);
    await i18n.changeLanguage("zh");
  });

  afterEach(() => vi.unstubAllGlobals());

  it("编辑时空 name 应回退到 id 回填输入框", async () => {
    renderWithI18n(<JsonForm mode="edit" id="glm" onCancel={() => {}} onSaved={() => {}} />);
    const nameInput = (await screen.findByTestId("provider-name-input")) as HTMLInputElement;
    await waitFor(() => expect(nameInput.value).toBe("glm"));
  });

  it("编辑时非空 name 应原样回填输入框", async () => {
    fetchMock.mockImplementation(async (url: string, init?: RequestInit) => {
      fetchCalls.push({ url, init });
      if (url.includes("/presets/deepseek")) return res(PRESET_DETAIL);
      if (url.includes("/presets")) return res(PRESET_LIST);
      if (url.includes("/providers/")) {
        return res({ id: "glm", name: "智谱 GLM", settings: { env: {} }, isolationMode: "" });
      }
      return res({});
    });
    renderWithI18n(<JsonForm mode="edit" id="glm" onCancel={() => {}} onSaved={() => {}} />);
    const nameInput = (await screen.findByTestId("provider-name-input")) as HTMLInputElement;
    await waitFor(() => expect(nameInput.value).toBe("智谱 GLM"));
  });

  it("创建时 name 输入框初始为空", async () => {
    renderWithI18n(<JsonForm mode="create" onCancel={() => {}} onSaved={() => {}} />);
    const nameInput = (await screen.findByTestId("provider-name-input")) as HTMLInputElement;
    expect(nameInput.value).toBe("");
  });

  it("选择 preset 后 API key 输入框不应显示 ${API_KEY} 占位符", async () => {
    renderWithI18n(<JsonForm mode="create" onCancel={() => {}} onSaved={() => {}} />);
    const presetSelect = (await screen.findByTestId("preset-select")) as HTMLSelectElement;
    await act(async () => {
      presetSelect.value = "deepseek";
      presetSelect.dispatchEvent(new Event("change", { bubbles: true }));
    });

    const apiKeyInput = (await screen.findByTestId("provider-api-key-input")) as HTMLInputElement;
    await waitFor(() => expect(apiKeyInput.value).toBe(""));
    expect(apiKeyInput.placeholder).toBe("sk-...");
  });

  it("选择 preset 后应自动填充 baseURL、model 和 JSON", async () => {
    renderWithI18n(<JsonForm mode="create" onCancel={() => {}} onSaved={() => {}} />);
    const presetSelect = (await screen.findByTestId("preset-select")) as HTMLSelectElement;
    await act(async () => {
      presetSelect.value = "deepseek";
      presetSelect.dispatchEvent(new Event("change", { bubbles: true }));
    });

    await waitFor(() => {
      const baseUrlInput = screen.getByTestId("env-field-ANTHROPIC_BASE_URL") as HTMLInputElement;
      expect(baseUrlInput.value).toBe("https://api.deepseek.com/anthropic");
    });

    const modelInput = screen.getByTestId("env-field-ANTHROPIC_MODEL") as HTMLInputElement;
    expect(modelInput.value).toBe("deepseek-v4-pro");

    const jsonTextarea = screen.getByTestId("provider-json-textarea") as HTMLTextAreaElement;
    expect(jsonTextarea.value).toContain("https://api.deepseek.com/anthropic");
    expect(jsonTextarea.value).toContain("deepseek-v4-pro");
  });

  it("切换 preset 后 JSON 应更新为新 preset 的默认配置", async () => {
    renderWithI18n(<JsonForm mode="create" onCancel={() => {}} onSaved={() => {}} />);
    const presetSelect = (await screen.findByTestId("preset-select")) as HTMLSelectElement;

    await act(async () => {
      presetSelect.value = "deepseek";
      presetSelect.dispatchEvent(new Event("change", { bubbles: true }));
    });
    await waitFor(() => {
      const baseUrlInput = screen.getByTestId("env-field-ANTHROPIC_BASE_URL") as HTMLInputElement;
      expect(baseUrlInput.value).toBe("https://api.deepseek.com/anthropic");
    });

    await act(async () => {
      presetSelect.value = "zhipu-glm";
      presetSelect.dispatchEvent(new Event("change", { bubbles: true }));
    });
    await waitFor(() => {
      const baseUrlInput = screen.getByTestId("env-field-ANTHROPIC_BASE_URL") as HTMLInputElement;
      expect(baseUrlInput.value).toBe("https://open.bigmodel.cn/api/anthropic");
    });

    const jsonTextarea = screen.getByTestId("provider-json-textarea") as HTMLTextAreaElement;
    expect(jsonTextarea.value).toContain("https://open.bigmodel.cn/api/anthropic");
    expect(jsonTextarea.value).toContain("glm-5.1");
  });

  it("API key 输入框可在明文/密文之间切换", async () => {
    const user = userEvent.setup();
    renderWithI18n(<JsonForm mode="create" onCancel={() => {}} onSaved={() => {}} />);
    const apiKeyInput = (await screen.findByTestId("provider-api-key-input")) as HTMLInputElement;
    const toggleButton = screen.getByTestId("provider-api-key-toggle");

    expect(apiKeyInput.type).toBe("password");
    await user.click(toggleButton);
    expect(apiKeyInput.type).toBe("text");
    await user.click(toggleButton);
    expect(apiKeyInput.type).toBe("password");
  });

  it("修改表单字段后 JSON 应同步更新", async () => {
    renderWithI18n(<JsonForm mode="create" onCancel={() => {}} onSaved={() => {}} />);
    const baseUrlInput = (await screen.findByTestId(
      "env-field-ANTHROPIC_BASE_URL",
    )) as HTMLInputElement;
    await act(async () => {
      await userEvent.clear(baseUrlInput);
      await userEvent.type(baseUrlInput, "https://example.com");
    });

    const jsonTextarea = screen.getByTestId("provider-json-textarea") as HTMLTextAreaElement;
    await waitFor(() => {
      expect(jsonTextarea.value).toContain("https://example.com");
    });
  });

  it("修改 JSON 后表单字段应同步更新", async () => {
    const user = userEvent.setup();
    renderWithI18n(<JsonForm mode="create" onCancel={() => {}} onSaved={() => {}} />);
    const jsonTextarea = (await screen.findByTestId(
      "provider-json-textarea",
    )) as HTMLTextAreaElement;
    const nextValue = JSON.stringify({ env: { ANTHROPIC_BASE_URL: "https://json-edited.com" } });
    await act(async () => {
      await user.clear(jsonTextarea);
      fireEvent.change(jsonTextarea, { target: { value: nextValue } });
    });

    const baseUrlInput = screen.getByTestId("env-field-ANTHROPIC_BASE_URL") as HTMLInputElement;
    await waitFor(() => {
      expect(baseUrlInput.value).toBe("https://json-edited.com");
    });
  });

  it("删除 API key 后 JSON 中仍保留 ANTHROPIC_AUTH_TOKEN 字段（必填字段不删除）", async () => {
    const user = userEvent.setup();
    renderWithI18n(<JsonForm mode="create" onCancel={() => {}} onSaved={() => {}} />);
    const apiKeyInput = (await screen.findByTestId("provider-api-key-input")) as HTMLInputElement;
    await act(async () => {
      await user.type(apiKeyInput, "sk-test");
      await user.clear(apiKeyInput);
    });

    const jsonTextarea = screen.getByTestId("provider-json-textarea") as HTMLTextAreaElement;
    await waitFor(() => {
      expect(jsonTextarea.value).toContain('"ANTHROPIC_AUTH_TOKEN"');
    });
    const parsed = JSON.parse(jsonTextarea.value);
    expect(parsed.env.ANTHROPIC_AUTH_TOKEN).toBe("");
  });

  it("清空 Base URL 后 JSON 中仍保留 ANTHROPIC_BASE_URL 字段", async () => {
    const user = userEvent.setup();
    renderWithI18n(<JsonForm mode="create" onCancel={() => {}} onSaved={() => {}} />);
    const baseUrlInput = (await screen.findByTestId(
      "env-field-ANTHROPIC_BASE_URL",
    )) as HTMLInputElement;
    await act(async () => {
      await user.type(baseUrlInput, "https://example.com");
      await user.clear(baseUrlInput);
    });

    const jsonTextarea = screen.getByTestId("provider-json-textarea") as HTMLTextAreaElement;
    await waitFor(() => {
      expect(jsonTextarea.value).toContain('"ANTHROPIC_BASE_URL"');
    });
    const parsed = JSON.parse(jsonTextarea.value);
    expect(parsed.env.ANTHROPIC_BASE_URL).toBe("");
  });

  it("必填字段为空时保存应提示校验错误", async () => {
    const user = userEvent.setup();
    const onSaved = vi.fn();
    renderWithI18n(<JsonForm mode="create" onCancel={() => {}} onSaved={onSaved} />);
    const idInput = (await screen.findByTestId("provider-id-input")) as HTMLInputElement;
    await user.type(idInput, "no-key");

    const saveButton = screen.getByTestId("provider-save-button");
    await user.click(saveButton);

    const error = await screen.findByTestId("provider-form-error");
    await waitFor(() => {
      expect(error).toBeVisible();
    });
    expect(error.textContent).toContain("ANTHROPIC_BASE_URL");
    expect(error.textContent).toContain("ANTHROPIC_AUTH_TOKEN");
    expect(onSaved).not.toHaveBeenCalled();
  });

  it("切换认证字段后，校验应针对新的认证字段", async () => {
    const user = userEvent.setup();
    const onSaved = vi.fn();
    renderWithI18n(<JsonForm mode="create" onCancel={() => {}} onSaved={onSaved} />);
    const idInput = (await screen.findByTestId("provider-id-input")) as HTMLInputElement;
    const baseUrlInput = screen.getByTestId("env-field-ANTHROPIC_BASE_URL") as HTMLInputElement;

    await act(async () => {
      await user.type(idInput, "switch-auth");
      await user.type(baseUrlInput, "https://example.com");
      await user.click(screen.getByTestId("toggle-advanced"));
    });

    const authFieldSelect = screen.getByTestId("provider-auth-field-select") as HTMLSelectElement;
    await act(async () => {
      authFieldSelect.value = "ANTHROPIC_API_KEY";
      authFieldSelect.dispatchEvent(new Event("change", { bubbles: true }));
    });

    const saveButton = screen.getByTestId("provider-save-button");
    await user.click(saveButton);

    const error = await screen.findByTestId("provider-form-error");
    await waitFor(() => {
      expect(error).toBeVisible();
    });
    // 校验的是新字段，而不是默认的 ANTHROPIC_AUTH_TOKEN。
    expect(error.textContent).toContain("ANTHROPIC_API_KEY");
    expect(error.textContent).not.toContain("ANTHROPIC_AUTH_TOKEN");
    expect(onSaved).not.toHaveBeenCalled();
  });

  it("在 preset 模式下切换认证字段，只校验新的认证字段", async () => {
    const user = userEvent.setup();
    const onSaved = vi.fn();
    renderWithI18n(<JsonForm mode="create" onCancel={() => {}} onSaved={onSaved} />);
    const idInput = (await screen.findByTestId("provider-id-input")) as HTMLInputElement;
    const presetSelect = screen.getByTestId("preset-select") as HTMLSelectElement;

    await act(async () => {
      await user.type(idInput, "preset-switch-auth");
      presetSelect.value = "deepseek";
      presetSelect.dispatchEvent(new Event("change", { bubbles: true }));
    });

    // 等待 preset 默认值填充完成。
    await waitFor(() => {
      expect((screen.getByTestId("env-field-ANTHROPIC_BASE_URL") as HTMLInputElement).value).toBe(
        "https://api.deepseek.com/anthropic",
      );
    });

    await act(async () => {
      await user.click(screen.getByTestId("toggle-advanced"));
    });

    const authFieldSelect = screen.getByTestId("provider-auth-field-select") as HTMLSelectElement;
    await act(async () => {
      authFieldSelect.value = "ANTHROPIC_API_KEY";
      authFieldSelect.dispatchEvent(new Event("change", { bubbles: true }));
    });

    const saveButton = screen.getByTestId("provider-save-button");
    await user.click(saveButton);

    const error = await screen.findByTestId("provider-form-error");
    await waitFor(() => {
      expect(error).toBeVisible();
    });
    // deepseek 默认 requiredVars 含 ANTHROPIC_AUTH_TOKEN，但切换后应只校验 ANTHROPIC_API_KEY。
    expect(error.textContent).toContain("ANTHROPIC_API_KEY");
    expect(error.textContent).not.toContain("ANTHROPIC_AUTH_TOKEN");
    expect(onSaved).not.toHaveBeenCalled();
  });

  it("保存时应提交 settings JSON", async () => {
    const onSaved = vi.fn();
    renderWithI18n(<JsonForm mode="create" onCancel={() => {}} onSaved={onSaved} />);
    const idInput = (await screen.findByTestId("provider-id-input")) as HTMLInputElement;
    const apiKeyInput = screen.getByTestId("provider-api-key-input") as HTMLInputElement;
    const baseUrlInput = screen.getByTestId("env-field-ANTHROPIC_BASE_URL") as HTMLInputElement;

    await act(async () => {
      await userEvent.type(idInput, "ds");
      await userEvent.type(apiKeyInput, "sk-ds");
      await userEvent.clear(baseUrlInput);
      await userEvent.type(baseUrlInput, "https://ds.example.com");
    });

    const saveButton = screen.getByTestId("provider-save-button");
    await act(async () => {
      await userEvent.click(saveButton);
    });

    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    const postCall = fetchCalls.find((c) => c.init?.method === "POST");
    expect(postCall).toBeDefined();
    const body = JSON.parse(postCall!.init!.body as string);
    expect(body.id).toBe("ds");
    expect(body.settings.env.ANTHROPIC_AUTH_TOKEN).toBe("sk-ds");
    expect(body.settings.env.ANTHROPIC_BASE_URL).toBe("https://ds.example.com");
  });
});
