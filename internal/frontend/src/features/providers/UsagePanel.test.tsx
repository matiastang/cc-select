import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nextProvider } from "react-i18next";
import i18n from "../../i18n";
import { UsagePanel } from "./UsagePanel";

function mockUsage(body: unknown) {
  const fn = vi.fn(async () => ({
    ok: true,
    status: 200,
    json: () => Promise.resolve(body),
  }));
  vi.stubGlobal("fetch", fn as unknown as typeof fetch);
  return fn;
}

function renderPanel(id = "glm") {
  render(
    <I18nextProvider i18n={i18n}>
      <UsagePanel providerId={id} />
    </I18nextProvider>,
  );
}

describe("UsagePanel（US7 / R11）", () => {
  it("手动刷新后展示窗口用量（5h 百分比 + resetAt）", async () => {
    mockUsage({
      supported: true,
      windows: [
        { type: "5h", percent: 63.5, resetAt: "2026-10-26T15:00:00Z" },
        { type: "week", used: 612, limit: 2400 },
      ],
    });
    renderPanel();
    fireEvent.click(screen.getByTestId("usage-refresh-button"));
    await waitFor(() => {
      expect(screen.getByTestId("usage-windows")).toBeTruthy();
    });
    expect(screen.getAllByRole("progressbar").length).toBe(2);
    expect(screen.getByTestId("usage-window-5h")).toHaveTextContent("63.5");
    expect(screen.getByTestId("usage-window-week")).toHaveTextContent("25.5");
    expect(screen.getByTestId("usage-fetched-at")).toBeTruthy();
  });

  it("不支持时优雅降级占位", async () => {
    mockUsage({ supported: false, reason: "no_limit_headers" });
    renderPanel("minimax");
    fireEvent.click(screen.getByTestId("usage-refresh-button"));
    await waitFor(() => {
      expect(screen.getByTestId("usage-unsupported")).toBeTruthy();
    });
  });

  it("查询失败显示错误与重试", async () => {
    mockUsage({ supported: false, error: "quota endpoint status 503" });
    renderPanel();
    fireEvent.click(screen.getByTestId("usage-refresh-button"));
    await waitFor(() => {
      expect(screen.getByTestId("usage-error")).toBeTruthy();
    });
    expect(screen.getByTestId("usage-refresh-button")).toBeTruthy();
  });

  it("仅手动触发——不自动发请求", () => {
    const fn = mockUsage({ supported: true, windows: [] });
    renderPanel();
    expect(fn).not.toHaveBeenCalled();
  });
});
