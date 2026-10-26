import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "../../i18n"; // 初始化 i18next（desktop 命名空间），否则 t() 返回原始键
import { DesktopSettings } from "./DesktopSettings";

// T018/T025：DesktopSettings 契约。mock 按 URL 路由 + 每路由独立序列，
// 避免 settings 请求加入后全局序列错位（风格沿 KeychainCard 的序列思路）。

type Resp = { ok: boolean; status: number; body: unknown };

function mockRoutes(routes: Record<string, Resp[]>) {
  const idx: Record<string, number> = {};
  const f = vi.fn(async (url: string, init?: RequestInit) => {
    for (const prefix of Object.keys(routes)) {
      if (url.includes(prefix)) {
        const i = idx[prefix] ?? 0;
        idx[prefix] = i + 1;
        const r = routes[prefix][Math.min(i, routes[prefix].length - 1)];
        void init;
        // 204 等 null-body 状态不能携带响应体（Response 构造约束）。
        const body = r.body === null ? null : JSON.stringify(r.body);
        return new Response(body, { status: r.status });
      }
    }
    return new Response("{}", { status: 404 });
  });
  vi.stubGlobal("fetch", f);
  return f;
}

const INFO = (cli: {
  installedPath: string | null;
  installedVersion: string | null;
  updateAvailable: boolean;
}) => ({
  mode: "desktop",
  appVersion: "0.1.0",
  embeddedCliVersion: "0.1.0",
  cli,
});
const DEFAULT_SETTINGS = { trayResident: true, autoStart: false, autoUpdateCheck: true };

describe("DesktopSettings", () => {
  beforeEach(() => vi.unstubAllGlobals());

  it("web 模式（info 404）渲染为空且不拉 settings", async () => {
    const f = mockRoutes({ "/desktop/info": [{ ok: false, status: 404, body: {} }] });
    const { container } = render(<DesktopSettings />);
    await waitFor(() => expect(container.querySelector("button")).toBeNull());
    expect(f.mock.calls.filter((c) => String(c[0]).includes("settings"))).toHaveLength(0);
  });

  it("未安装态显示安装按钮，点击成功后显示结果并刷新", async () => {
    const f = mockRoutes({
      "/desktop/info": [
        {
          ok: true,
          status: 200,
          body: INFO({ installedPath: null, installedVersion: null, updateAvailable: false }),
        },
        {
          ok: true,
          status: 200,
          body: INFO({
            installedPath: "/usr/local/bin/cc-select",
            installedVersion: "0.1.0",
            updateAvailable: false,
          }),
        },
      ],
      "/desktop/settings": [{ ok: true, status: 200, body: DEFAULT_SETTINGS }],
      "/cli/install": [
        {
          ok: true,
          status: 200,
          body: { action: "installed", installedPath: "/usr/local/bin/cc-select" },
        },
      ],
    });
    render(<DesktopSettings />);
    fireEvent.click(await waitFor(() => screen.getByTestId("desktop-cli-install")));
    await waitFor(() => expect(screen.getByTestId("desktop-cli-notice")).toBeVisible());
    const inst = f.mock.calls.find((c) => String(c[0]).includes("cli/install")) as unknown as [
      string,
      RequestInit,
    ];
    expect(inst[1].body).toBe('{"confirmOverwrite":false}');
  });

  it("外部二进制 409 → 覆盖确认，确认后带 confirmOverwrite:true 重发", async () => {
    const f = mockRoutes({
      "/desktop/info": [
        {
          ok: true,
          status: 200,
          body: INFO({
            installedPath: "/opt/x/cc-select",
            installedVersion: "unknown",
            updateAvailable: true,
          }),
        },
      ],
      "/desktop/settings": [{ ok: true, status: 200, body: DEFAULT_SETTINGS }],
      "/cli/install": [
        {
          ok: false,
          status: 409,
          body: { conflict: "external-binary", installedPath: "/opt/x/cc-select" },
        },
        {
          ok: true,
          status: 200,
          body: { action: "overwritten", installedPath: "/opt/x/cc-select" },
        },
      ],
    });
    render(<DesktopSettings />);
    fireEvent.click(await waitFor(() => screen.getByTestId("desktop-cli-install")));
    await waitFor(() => expect(screen.getByTestId("desktop-cli-external")).toBeVisible());
    fireEvent.click(screen.getByTestId("desktop-cli-overwrite"));
    await waitFor(() => expect(screen.getByTestId("desktop-cli-notice")).toBeVisible());
    const puts = f.mock.calls.filter((c) => String(c[0]).includes("cli/install")) as unknown as [
      string,
      RequestInit,
    ][];
    expect(JSON.parse(puts[1][1].body as string).confirmOverwrite).toBe(true);
  });

  it("权限 409 → 展示指引不误报成功", async () => {
    mockRoutes({
      "/desktop/info": [
        {
          ok: true,
          status: 200,
          body: INFO({ installedPath: null, installedVersion: null, updateAvailable: false }),
        },
      ],
      "/desktop/settings": [{ ok: true, status: 200, body: DEFAULT_SETTINGS }],
      "/cli/install": [
        {
          ok: false,
          status: 409,
          body: { conflict: "permission", hint: "no writable install directory" },
        },
      ],
    });
    render(<DesktopSettings />);
    fireEvent.click(await waitFor(() => screen.getByTestId("desktop-cli-install")));
    await waitFor(() => expect(screen.getByTestId("desktop-cli-permission")).toBeVisible());
    expect(screen.queryByTestId("desktop-cli-notice")).toBeNull();
  });

  it("已装旧版显示升级文案", async () => {
    mockRoutes({
      "/desktop/info": [
        {
          ok: true,
          status: 200,
          body: INFO({
            installedPath: "/usr/local/bin/cc-select",
            installedVersion: "0.0.7",
            updateAvailable: true,
          }),
        },
      ],
      "/desktop/settings": [{ ok: true, status: 200, body: DEFAULT_SETTINGS }],
    });
    render(<DesktopSettings />);
    const btn = await waitFor(() => screen.getByTestId("desktop-cli-install"));
    expect(btn.textContent).toContain("0.1.0");
  });

  it("自启开关切换，503 时回滚（服务同步失败场景）", async () => {
    mockRoutes({
      "/desktop/info": [
        {
          ok: true,
          status: 200,
          body: INFO({ installedPath: null, installedVersion: null, updateAvailable: false }),
        },
      ],
      "/desktop/settings": [
        { ok: true, status: 200, body: DEFAULT_SETTINGS },
        { ok: false, status: 503, body: { error: "autostart unavailable" } },
      ],
    });
    render(<DesktopSettings />);
    const box = await waitFor(
      () => screen.getByTestId("desktop-autostart-toggle") as HTMLInputElement,
    );
    expect(box.checked).toBe(false);
    fireEvent.click(box);
    await waitFor(() => expect(screen.getByTestId("desktop-cli-error")).toBeVisible());
    expect(box.checked).toBe(false); // 回滚
  });

  it("托盘驻留开关可切换并 PUT 全量", async () => {
    const f = mockRoutes({
      "/desktop/info": [
        {
          ok: true,
          status: 200,
          body: INFO({ installedPath: null, installedVersion: null, updateAvailable: false }),
        },
      ],
      "/desktop/settings": [
        { ok: true, status: 200, body: DEFAULT_SETTINGS },
        { ok: true, status: 204, body: null },
      ],
    });
    render(<DesktopSettings />);
    const box = await waitFor(() => screen.getByTestId("desktop-tray-toggle") as HTMLInputElement);
    expect(box.checked).toBe(true);
    fireEvent.click(box);
    await waitFor(() => expect(box.checked).toBe(false));
    const put = f.mock.calls.find(
      (c) =>
        String(c[0]).includes("settings") && (c[1] as RequestInit | undefined)?.method === "PUT",
    ) as unknown as [string, RequestInit];
    expect(JSON.parse(put[1].body as string)).toEqual({
      trayResident: false,
      autoStart: false,
      autoUpdateCheck: true,
    });
  });

  it("桌面模式加载 settings，开关反映服务端值", async () => {
    mockRoutes({
      "/desktop/info": [
        {
          ok: true,
          status: 200,
          body: INFO({ installedPath: null, installedVersion: null, updateAvailable: false }),
        },
      ],
      "/desktop/settings": [
        { ok: true, status: 200, body: { ...DEFAULT_SETTINGS, autoUpdateCheck: false } },
      ],
    });
    render(<DesktopSettings />);
    const box = await waitFor(
      () => screen.getByTestId("desktop-autoupdate-toggle") as HTMLInputElement,
    );
    expect(box.checked).toBe(false);
  });

  it("切换开关 PUT 全量对象，成功后更新", async () => {
    const f = mockRoutes({
      "/desktop/info": [
        {
          ok: true,
          status: 200,
          body: INFO({ installedPath: null, installedVersion: null, updateAvailable: false }),
        },
      ],
      "/desktop/settings": [
        { ok: true, status: 200, body: DEFAULT_SETTINGS }, // GET
        { ok: true, status: 204, body: null }, // PUT 204
      ],
    });
    render(<DesktopSettings />);
    const box = await waitFor(
      () => screen.getByTestId("desktop-autoupdate-toggle") as HTMLInputElement,
    );
    fireEvent.click(box);
    await waitFor(() => expect(box.checked).toBe(false));
    const put = f.mock.calls.find(
      (c) =>
        String(c[0]).includes("settings") && (c[1] as RequestInit | undefined)?.method === "PUT",
    ) as unknown as [string, RequestInit];
    expect(JSON.parse(put[1].body as string)).toEqual({
      trayResident: true,
      autoStart: false,
      autoUpdateCheck: false,
    });
  });

  it("PUT 503（服务同步失败）→ 显示错误且开关回滚", async () => {
    mockRoutes({
      "/desktop/info": [
        {
          ok: true,
          status: 200,
          body: INFO({ installedPath: null, installedVersion: null, updateAvailable: false }),
        },
      ],
      "/desktop/settings": [
        { ok: true, status: 200, body: DEFAULT_SETTINGS },
        { ok: false, status: 503, body: { error: "autostart unavailable" } },
      ],
    });
    render(<DesktopSettings />);
    const box = await waitFor(
      () => screen.getByTestId("desktop-autoupdate-toggle") as HTMLInputElement,
    );
    fireEvent.click(box);
    await waitFor(() => expect(screen.getByTestId("desktop-cli-error")).toBeVisible());
    expect(box.checked).toBe(true); // 回滚：服务端未落盘
  });
});
