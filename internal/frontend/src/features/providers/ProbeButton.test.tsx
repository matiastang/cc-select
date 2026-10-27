import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nextProvider } from "react-i18next";
import i18n from "../../i18n";
import { ProbeButton } from "./ProbeButton";

function renderButton(env: () => Record<string, string>, body: unknown, status = 200) {
  const fn = vi.fn(async () => ({
    ok: status < 400,
    status,
    json: () => Promise.resolve(body),
  }));
  vi.stubGlobal("fetch", fn as unknown as typeof fetch);
  render(
    <I18nextProvider i18n={i18n}>
      <ProbeButton env={env} />
    </I18nextProvider>,
  );
  return fn;
}

describe("ProbeButton（US6 / FR-016-017）", () => {
  it("成功显示延迟与模型回显", async () => {
    renderButton(
      () => ({ ANTHROPIC_BASE_URL: "https://x", ANTHROPIC_AUTH_TOKEN: "sk", ANTHROPIC_MODEL: "m" }),
      { ok: true, latencyMs: 412, model: "glm-5.3" },
    );
    fireEvent.click(screen.getByTestId("probe-test-button"));
    await waitFor(() => {
      expect(screen.getByTestId("probe-result")).toHaveTextContent("glm-5.3");
    });
    expect(screen.getByTestId("probe-result")).toHaveTextContent("412");
  });

  it("失败显示分类错误", async () => {
    renderButton(
      () => ({
        ANTHROPIC_BASE_URL: "https://x",
        ANTHROPIC_AUTH_TOKEN: "bad",
        ANTHROPIC_MODEL: "m",
      }),
      { ok: false, error: { kind: "auth", message: "401 invalid api key" } },
    );
    fireEvent.click(screen.getByTestId("probe-test-button"));
    await waitFor(() => {
      expect(screen.getByTestId("probe-result")).toHaveTextContent("auth");
    });
  });

  it("必填缺失时禁用", () => {
    renderButton(() => ({ ANTHROPIC_BASE_URL: "https://x" }), {});
    expect(screen.getByTestId("probe-test-button")).toBeDisabled();
  });

  it("进行中显示加载态", async () => {
    let resolveFetch: (v: unknown) => void = () => {};
    const fn = vi.fn(
      () =>
        new Promise((r) => {
          resolveFetch = r;
        }),
    );
    vi.stubGlobal("fetch", fn as unknown as typeof fetch);
    render(
      <I18nextProvider i18n={i18n}>
        <ProbeButton
          env={() => ({
            ANTHROPIC_BASE_URL: "https://x",
            ANTHROPIC_AUTH_TOKEN: "sk",
            ANTHROPIC_MODEL: "m",
          })}
        />
      </I18nextProvider>,
    );
    fireEvent.click(screen.getByTestId("probe-test-button"));
    expect(screen.getByTestId("probe-test-button")).toBeDisabled();
    resolveFetch({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ ok: true, latencyMs: 1, model: "m" }),
    });
    await waitFor(() => expect(screen.getByTestId("probe-result")).toBeTruthy());
  });
});
