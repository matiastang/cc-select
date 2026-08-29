import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { KeychainCard } from "./KeychainCard";

function mockFetchSequence(responses: { ok: boolean; status: number; body: unknown }[]) {
  let i = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => {
      const r = responses[Math.min(i, responses.length - 1)];
      i++;
      return new Response(JSON.stringify(r.body), { status: r.status });
    }),
  );
}

describe("KeychainCard", () => {
  beforeEach(() => vi.unstubAllGlobals());

  it("开启失败（500）时复选框保持原状态并显示错误", async () => {
    mockFetchSequence([
      { ok: true, status: 200, body: { enabled: false } }, // 初始 GET
      { ok: false, status: 500, body: { error: "boom" } }, // PUT 失败
    ]);
    render(<KeychainCard />);
    const box = screen.getByTestId("keychain-toggle") as HTMLInputElement;
    await waitFor(() => expect(box.disabled).toBe(false));
    fireEvent.click(box);
    await waitFor(() => expect(screen.getByTestId("keychain-error")).toBeVisible());
    expect(box.checked).toBe(false); // 未被误置为已开启
  });
});
