import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import i18n from "../i18n";
import { JsonForm } from "./JsonForm";

function renderWithI18n(ui: React.ReactNode) {
  return render(<I18nextProvider i18n={i18n}>{ui}</I18nextProvider>);
}

function res(body: Record<string, unknown>, ok = true) {
  return { ok, json: () => Promise.resolve(body) };
}

describe("JsonForm", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock as unknown as typeof fetch);
    await i18n.changeLanguage("zh");
  });

  afterEach(() => vi.unstubAllGlobals());

  it("编辑时空 name 应回退到 id 回填输入框", async () => {
    fetchMock.mockResolvedValue(
      res({ id: "glm", name: "", settings: {}, isolationMode: "" })
    );
    renderWithI18n(
      <JsonForm
        mode="edit"
        id="glm"
        onCancel={() => {}}
        onSaved={() => {}}
      />
    );

    const nameInput = screen.getByTestId("provider-name-input") as HTMLInputElement;
    await waitFor(() => expect(nameInput.value).toBe("glm"));
  });

  it("编辑时非空 name 应原样回填输入框", async () => {
    fetchMock.mockResolvedValue(
      res({ id: "glm", name: "智谱 GLM", settings: {}, isolationMode: "" })
    );
    renderWithI18n(
      <JsonForm
        mode="edit"
        id="glm"
        onCancel={() => {}}
        onSaved={() => {}}
      />
    );

    const nameInput = screen.getByTestId("provider-name-input") as HTMLInputElement;
    await waitFor(() => expect(nameInput.value).toBe("智谱 GLM"));
  });

  it("创建时 name 输入框初始为空", () => {
    renderWithI18n(
      <JsonForm mode="create" onCancel={() => {}} onSaved={() => {}} />
    );

    const nameInput = screen.getByTestId("provider-name-input") as HTMLInputElement;
    expect(nameInput.value).toBe("");
  });
});
