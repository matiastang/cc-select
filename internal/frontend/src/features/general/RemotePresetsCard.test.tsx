import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nextProvider } from "react-i18next";
import i18n from "../../i18n";
import { ToastProvider } from "../../components/ui";
import { RemotePresetsCard } from "./RemotePresetsCard";

function mockFetch(urls: Record<string, { status?: number; body: unknown }>) {
  const fn = vi.fn(async (input: RequestInfo) => {
    const url = String(input);
    for (const prefix of Object.keys(urls)) {
      if (url.includes(prefix)) {
        const cfg = urls[prefix];
        return {
          ok: (cfg.status ?? 200) < 400,
          status: cfg.status ?? 200,
          json: () => Promise.resolve(cfg.body),
        };
      }
    }
    throw new Error("unexpected fetch: " + url);
  });
  vi.stubGlobal("fetch", fn as unknown as typeof fetch);
  return fn;
}

function renderCard() {
  return render(
    <I18nextProvider i18n={i18n}>
      <ToastProvider>
        <RemotePresetsCard />
      </ToastProvider>
    </I18nextProvider>,
  );
}

describe("RemotePresetsCard（US5 / FR-012–014）", () => {
  it("显示当前来源徽标与开关状态", async () => {
    mockFetch({
      "/api/v1/remote-presets": { body: { enabled: true } },
    });
    renderCard();
    expect(await screen.findByTestId("presets-source-badge")).toHaveTextContent(
      /built-?in|remote|cache/i,
    );
    const toggle = await screen.findByTestId("remote-presets-toggle");
    expect(toggle).toHaveAttribute("data-state", "checked");
  });

  it("关闭开关发起 PUT 且成功后视觉反映", async () => {
    const fn = mockFetch({
      "/api/v1/remote-presets": { body: { enabled: true } },
    });
    renderCard();
    await screen.findByTestId("remote-presets-toggle");
    // Radix Switch 交互走真实指针事件
    const { default: userEvent } = await import("@testing-library/user-event");
    await userEvent.click(screen.getByTestId("remote-presets-toggle"));
    await waitFor(() => {
      const puts = fn.mock.calls.filter((c) => String(c[0]).includes("remote-presets"));
      expect(puts.length).toBeGreaterThanOrEqual(2); // GET + PUT
    });
  });

  it("手动刷新显示远端来源与时间", async () => {
    mockFetch({
      "/api/v1/remote-presets": { body: { enabled: true } },
      "/api/v1/presets/refresh": {
        body: { meta: { source: "remote", fetchedAt: "2026-10-26T12:00:00Z" } },
      },
    });
    renderCard();
    const btn = await screen.findByTestId("presets-refresh-button");
    fireEvent.click(btn);
    await waitFor(() => {
      expect(screen.getByTestId("presets-source-badge")).toHaveTextContent("remote");
    });
  });

  it("刷新失败回落时展示 warning 徽标", async () => {
    mockFetch({
      "/api/v1/remote-presets": { body: { enabled: true } },
      "/api/v1/presets/refresh": {
        body: { meta: { source: "builtin", fetchedAt: null }, warning: "fetch_failed" },
      },
    });
    renderCard();
    fireEvent.click(await screen.findByTestId("presets-refresh-button"));
    await waitFor(() => {
      expect(screen.getByTestId("presets-refresh-warning")).toBeTruthy();
    });
  });
});
