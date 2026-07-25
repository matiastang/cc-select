import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import i18n from "./i18n";
import { ShellIntegrationBanner } from "./ShellIntegrationBanner";

// 组件单测覆盖 ShellIntegrationBanner 的状态机/字段映射/shell 转发/错误处理。
// 真实的浏览器交互（含 Go embed 的后端）由 e2e/providers.spec.ts 覆盖。

// 构造一个最小 fetch Response 形状（ok + json()）。
function res(body: Record<string, unknown>, ok = true) {
  return { ok, json: () => Promise.resolve(body) };
}

type Status = Partial<{
  supported: boolean;
  installed: boolean;
  legacy: boolean;
  shell: string;
  canAutoInstall: boolean;
}>;

const getBody = (o: Status = {}) => ({
  supported: true,
  installed: false,
  legacy: false,
  shell: "zsh",
  canAutoInstall: true,
  ...o,
});

function renderWithI18n(ui: React.ReactNode) {
  return render(<I18nextProvider i18n={i18n}>{ui}</I18nextProvider>);
}

describe("ShellIntegrationBanner", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock as unknown as typeof fetch);
    await i18n.changeLanguage("zh");
  });

  afterEach(() => vi.unstubAllGlobals());

  it("已安装时不渲染", async () => {
    fetchMock.mockResolvedValue(res(getBody({ installed: true })));
    const { container } = renderWithI18n(<ShellIntegrationBanner />);
    await waitFor(() => expect(container.querySelector(".notice")).toBeNull());
  });

  it("不支持的 shell 显示「暂不支持」提示", async () => {
    fetchMock.mockResolvedValue(res(getBody({ supported: false, shell: "fish" })));
    renderWithI18n(<ShellIntegrationBanner />);
    await waitFor(() => expect(screen.getByText(/暂不支持/)).toBeInTheDocument());
    expect(screen.getByText(/fish/)).toBeInTheDocument();
  });

  it("legacy 旧版显示「建议升级」", async () => {
    fetchMock.mockResolvedValue(res(getBody({ legacy: true })));
    renderWithI18n(<ShellIntegrationBanner />);
    await waitFor(() => expect(screen.getByText(/建议升级/)).toBeInTheDocument());
  });

  it("needed 显示「一键安装」按钮与 shell 名", async () => {
    fetchMock.mockResolvedValue(res(getBody({ shell: "bash" })));
    renderWithI18n(<ShellIntegrationBanner />);
    const btn = await screen.findByRole("button", { name: "一键安装" });
    expect(btn).toBeInTheDocument();
    expect(screen.getByText(/bash/)).toBeInTheDocument();
  });

  it("点击安装成功(appended)进入 done", async () => {
    fetchMock
      .mockResolvedValueOnce(res(getBody({ shell: "zsh" })))
      .mockResolvedValueOnce(
        res({ action: "appended", shell: "zsh", rcPath: "/x/.zshrc", message: "已写入" }),
      );
    renderWithI18n(<ShellIntegrationBanner />);
    fireEvent.click(await screen.findByRole("button", { name: "一键安装" }));
    await waitFor(() => expect(screen.getByText(/✅/)).toBeInTheDocument());
    expect(screen.getByText(/已写入/)).toBeInTheDocument();
  });

  it("manual 降级展示 snippet 文本框", async () => {
    fetchMock.mockResolvedValueOnce(res(getBody({}))).mockResolvedValueOnce(
      res({
        action: "manual",
        shell: "powershell",
        snippet: "function ccs { }",
        message: "复制粘贴",
      }),
    );
    renderWithI18n(<ShellIntegrationBanner />);
    fireEvent.click(await screen.findByRole("button", { name: "一键安装" }));
    await waitFor(() => expect(screen.getByText("需要手动完成 shell 集成")).toBeInTheDocument());
    expect(screen.getByRole("textbox")).toHaveValue("function ccs { }");
  });

  it("POST 随请求回传 shell 字段（GET/POST 同一 shell）", async () => {
    fetchMock
      .mockResolvedValueOnce(res(getBody({ shell: "bash" })))
      .mockResolvedValueOnce(
        res({ action: "appended", shell: "bash", rcPath: "/x", message: "ok" }),
      );
    renderWithI18n(<ShellIntegrationBanner />);
    fireEvent.click(await screen.findByRole("button", { name: "一键安装" }));
    await waitFor(() => expect(screen.getByText(/✅/)).toBeInTheDocument());
    const post = fetchMock.mock.calls[1];
    expect(post[0]).toBe("/api/v1/shell-integration/install");
    expect(JSON.parse(post[1].body).shell).toBe("bash");
  });

  it("安装失败显示错误并回到 needed", async () => {
    fetchMock
      .mockResolvedValueOnce(res(getBody({ shell: "zsh" })))
      .mockResolvedValueOnce(res({ error: "写失败啦" }, false));
    renderWithI18n(<ShellIntegrationBanner />);
    fireEvent.click(await screen.findByRole("button", { name: "一键安装" }));
    await waitFor(() => expect(screen.getByText(/写失败啦/)).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "一键安装" })).toBeInTheDocument();
  });

  // ---- PowerShell 多 target（PS7 + PS5.1 并存）----

  const psTargets = (installed7: boolean, installed5: boolean) => ({
    supported: true,
    installed: installed7 || installed5,
    legacy: false,
    shell: "powershell",
    targets: [
      { id: "powershell7", rcPath: "/Documents/PowerShell/profile.ps1", installed: installed7 },
      {
        id: "powershell5",
        rcPath: "/Documents/WindowsPowerShell/profile.ps1",
        installed: installed5,
      },
    ],
  });

  it("多 target 都未装：显示两个可点按钮 + 说明文案", async () => {
    fetchMock.mockResolvedValue(res(psTargets(false, false)));
    renderWithI18n(<ShellIntegrationBanner />);
    const ps7 = await screen.findByTestId("shell-install-button-powershell7");
    const ps5 = screen.getByTestId("shell-install-button-powershell5");
    expect(ps7).not.toBeDisabled();
    expect(ps5).not.toBeDisabled();
    expect(screen.getByText(/分别安装/)).toBeInTheDocument(); // multiHint 说明两 profile 不同
  });

  it("多 target 其中一个已装：对应按钮置灰，另一个仍可点", async () => {
    fetchMock.mockResolvedValue(res(psTargets(true, false)));
    renderWithI18n(<ShellIntegrationBanner />);
    const ps7 = await screen.findByTestId("shell-install-button-powershell7");
    const ps5 = screen.getByTestId("shell-install-button-powershell5");
    expect(ps7).toBeDisabled(); // 已装 → 置灰
    expect(ps5).not.toBeDisabled();
  });

  it("多 target 都已装：隐藏整个模块", async () => {
    fetchMock.mockResolvedValue(res(psTargets(true, true)));
    const { container } = renderWithI18n(<ShellIntegrationBanner />);
    await waitFor(() => expect(container.querySelector(".notice")).toBeNull());
  });

  it("多 target 点击安装回传 target 字段，并刷新置灰", async () => {
    fetchMock
      .mockResolvedValueOnce(res(psTargets(false, false))) // 初始 GET
      .mockResolvedValueOnce(
        res({ action: "appended", shell: "powershell", rcPath: "/ps7", message: "ok" }),
      ) // POST
      .mockResolvedValueOnce(res(psTargets(true, false))); // 装完刷新 GET
    renderWithI18n(<ShellIntegrationBanner />);
    const ps7 = await screen.findByTestId("shell-install-button-powershell7");
    fireEvent.click(ps7);
    const post = fetchMock.mock.calls[1];
    expect(post[0]).toBe("/api/v1/shell-integration/install");
    expect(JSON.parse(post[1].body).target).toBe("powershell7");
    // 装完后 PS7 置灰、PS5 仍可点
    await waitFor(() =>
      expect(screen.getByTestId("shell-install-button-powershell7")).toBeDisabled(),
    );
    expect(screen.getByTestId("shell-install-button-powershell5")).not.toBeDisabled();
  });
});
