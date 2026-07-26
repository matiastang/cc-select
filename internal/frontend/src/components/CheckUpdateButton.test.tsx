import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import i18n from "../i18n";
import { CheckUpdateButton } from "./CheckUpdateButton";

// 组件单测覆盖 CheckUpdateButton 的状态机：
// 无更新隐藏 / 有更新显示按钮 / 安装成功显示重启卡片 / refused 显示精确指引 / 错误显示错误卡片。
// 真实浏览器交互（含 Go embed 后端）由 e2e 覆盖。

// 构造最小 fetch Response 形状（ok + json()）。
function res(body: Record<string, unknown>, ok = true) {
  return { ok, status: ok ? 200 : 409, json: () => Promise.resolve(body) };
}

function renderWithI18n(ui: React.ReactNode) {
  return render(<I18nextProvider i18n={i18n}>{ui}</I18nextProvider>);
}

describe("CheckUpdateButton", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock as unknown as typeof fetch);
    await i18n.changeLanguage("zh");
  });

  afterEach(() => vi.unstubAllGlobals());

  it("无更新时不渲染任何内容", async () => {
    fetchMock.mockResolvedValue(res({ hasUpdate: false, currentVersion: "1.0.0" }));
    const { container } = renderWithI18n(<CheckUpdateButton />);
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    await waitFor(() => expect(container.querySelector("button")).toBeNull());
  });

  it("检查请求失败时静默隐藏", async () => {
    fetchMock.mockRejectedValue(new Error("network down"));
    const { container } = renderWithI18n(<CheckUpdateButton />);
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    expect(container.querySelector("button")).toBeNull();
  });

  it("有更新时显示「立即更新」按钮", async () => {
    fetchMock.mockResolvedValue(
      res({ hasUpdate: true, currentVersion: "1.0.0", latestVersion: "1.2.0" }),
    );
    renderWithI18n(<CheckUpdateButton />);
    const btn = await screen.findByTestId("update-now-button");
    expect(btn).toHaveTextContent("1.2.0");
  });

  it("点击更新成功后显示重启提示卡片", async () => {
    fetchMock
      .mockResolvedValueOnce(
        res({ hasUpdate: true, currentVersion: "1.0.0", latestVersion: "1.2.0" }),
      )
      .mockResolvedValueOnce(
        res({
          status: "installed",
          fromVersion: "1.0.0",
          toVersion: "1.2.0",
          restartRequired: true,
        }),
      );
    renderWithI18n(<CheckUpdateButton />);
    fireEvent.click(await screen.findByTestId("update-now-button"));
    const card = await screen.findByTestId("update-installed-card");
    expect(card).toHaveTextContent("已更新到 v1.2.0");
    expect(card).toHaveTextContent("cc-select gui");
  });

  it("refused(homebrew) 显示 brew 升级指引而非错误", async () => {
    fetchMock
      .mockResolvedValueOnce(
        res({ hasUpdate: true, currentVersion: "1.0.0", latestVersion: "1.2.0" }),
      )
      .mockResolvedValueOnce(res({ refused: true, kind: "homebrew", error: "..." }, false));
    renderWithI18n(<CheckUpdateButton />);
    fireEvent.click(await screen.findByTestId("update-now-button"));
    const card = await screen.findByTestId("update-refused-card");
    expect(card).toHaveTextContent("brew upgrade cc-select");
  });

  it("refused(dev) 显示 dev 构建指引", async () => {
    fetchMock
      .mockResolvedValueOnce(
        res({ hasUpdate: true, currentVersion: "1.0.0", latestVersion: "1.2.0" }),
      )
      .mockResolvedValueOnce(res({ refused: true, kind: "dev", error: "..." }, false));
    renderWithI18n(<CheckUpdateButton />);
    fireEvent.click(await screen.findByTestId("update-now-button"));
    expect(await screen.findByTestId("update-refused-card")).toHaveTextContent("dev 构建");
  });

  it("有更新且带 htmlUrl 时显示更新日志链接", async () => {
    fetchMock.mockResolvedValue(
      res({
        hasUpdate: true,
        currentVersion: "1.0.0",
        latestVersion: "1.2.0",
        htmlUrl: "https://github.com/matiastang/cc-select/releases/tag/v1.2.0",
      }),
    );
    renderWithI18n(<CheckUpdateButton />);
    await screen.findByTestId("update-now-button");
    const link = await screen.findByTestId("update-changelog-link");
    expect(link).toHaveAttribute(
      "href",
      "https://github.com/matiastang/cc-select/releases/tag/v1.2.0",
    );
    expect(link).toHaveAttribute("target", "_blank");
  });

  it("无 htmlUrl 时不渲染更新日志链接", async () => {
    fetchMock.mockResolvedValue(
      res({ hasUpdate: true, currentVersion: "1.0.0", latestVersion: "1.2.0" }),
    );
    renderWithI18n(<CheckUpdateButton />);
    await screen.findByTestId("update-now-button");
    expect(screen.queryByTestId("update-changelog-link")).toBeNull();
  });

  it("更新成功后卡片内含更新日志链接", async () => {
    fetchMock
      .mockResolvedValueOnce(
        res({
          hasUpdate: true,
          currentVersion: "1.0.0",
          latestVersion: "1.2.0",
          htmlUrl: "https://github.com/matiastang/cc-select/releases/tag/v1.2.0",
        }),
      )
      .mockResolvedValueOnce(
        res({
          status: "installed",
          fromVersion: "1.0.0",
          toVersion: "1.2.0",
          restartRequired: true,
        }),
      );
    renderWithI18n(<CheckUpdateButton />);
    fireEvent.click(await screen.findByTestId("update-now-button"));
    const card = await screen.findByTestId("update-installed-card");
    const link = await screen.findByTestId("update-changelog-link");
    expect(card).toContainElement(link);
  });

  it("安装失败显示错误卡片", async () => {
    fetchMock
      .mockResolvedValueOnce(
        res({ hasUpdate: true, currentVersion: "1.0.0", latestVersion: "1.2.0" }),
      )
      .mockResolvedValueOnce(res({ error: "boom" }, false));
    renderWithI18n(<CheckUpdateButton />);
    fireEvent.click(await screen.findByTestId("update-now-button"));
    expect(await screen.findByTestId("update-error-card")).toHaveTextContent("boom");
  });
});
