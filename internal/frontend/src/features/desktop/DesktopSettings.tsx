import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Card } from "../../components/ui";

// DesktopSettings 是桌面端专属设置区（US2 起步：CLI 安装；US3–US5 递增开关项）。
// 能力探测契约（contracts/desktop-api.md §3.1）：GET /desktop/info 在 web 模式（ccs gui）
// 下 404——组件渲染 null，实现单一前端双形态（FR-015）。

interface DesktopInfo {
  mode: string;
  appVersion: string;
  embeddedCliVersion: string;
  cli: {
    installedPath: string | null;
    installedVersion: string | null;
    updateAvailable: boolean;
  };
}

interface InstallConflict {
  conflict: string;
  installedPath?: string;
  hint?: string;
}

export function DesktopSettings() {
  const { t } = useTranslation("desktop");
  const [info, setInfo] = useState<DesktopInfo | null>(null);
  const [hidden, setHidden] = useState(false);
  const [busy, setBusy] = useState(false);
  const [conflict, setConflict] = useState<InstallConflict | null>(null);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  // 桌面偏好（US3 起：autoUpdateCheck；US4/US5 增 trayResident/autoStart 开关）。
  const [settings, setSettings] = useState<DesktopSettingsState>({
    trayResident: true,
    autoStart: false,
    autoUpdateCheck: true,
  });

  interface DesktopSettingsState {
    trayResident: boolean;
    autoStart: boolean;
    autoUpdateCheck: boolean;
  }

  // 返回是否桌面模式（false = web 形态，调用方据此跳过桌面专属请求）。
  const loadInfo = useCallback(async (): Promise<boolean> => {
    const r = await fetch("./api/v1/desktop/info");
    if (!r.ok) {
      setHidden(true); // 404 = web 模式；其他失败也保守隐藏
      return false;
    }
    setInfo((await r.json()) as DesktopInfo);
    return true;
  }, []);

  useEffect(() => {
    void (async () => {
      try {
        if (!(await loadInfo())) return; // web 模式：不发桌面专属请求
        const r = await fetch("./api/v1/desktop/settings");
        if (r.ok) setSettings((await r.json()) as DesktopSettingsState);
      } catch {
        setHidden(true);
      }
    })();
  }, [loadInfo]);

  // toggleSetting PUT 全量对象（契约 §3.2）；失败回滚显示态（服务端未落盘）。
  const toggleSetting = async (key: keyof DesktopSettingsState) => {
    const next = { ...settings, [key]: !settings[key] };
    setBusy(true);
    setError("");
    try {
      const r = await fetch("./api/v1/desktop/settings", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(next),
      });
      if (r.status === 503) {
        setError(t("settings.syncFailed"));
        return; // 不更新本地态（服务端整体回滚）
      }
      if (!r.ok) {
        setError(t("settings.saveFailed"));
        return;
      }
      setSettings(next);
    } catch {
      setError(t("settings.saveFailed"));
    } finally {
      setBusy(false);
    }
  };

  const install = async (confirmOverwrite: boolean) => {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const r = await fetch("./api/v1/desktop/cli/install", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ confirmOverwrite }),
      });
      if (r.status === 409) {
        setConflict((await r.json()) as InstallConflict);
        return;
      }
      if (!r.ok) {
        setError(t("cli.installFailed"));
        return;
      }
      const res = (await r.json()) as { action: string; installedPath: string; hint?: string };
      setConflict(null);
      setNotice(
        t("cli.installedOk", { action: res.action, path: res.installedPath }) +
          (res.hint ? `——${res.hint}` : ""),
      );
      await loadInfo();
    } catch {
      setError(t("cli.installFailed"));
    } finally {
      setBusy(false);
    }
  };

  if (hidden || !info) return null;

  const { cli } = info;
  const external = cli.installedVersion === "unknown";
  const buttonLabel = !cli.installedPath
    ? t("cli.install")
    : external
      ? t("cli.install")
      : cli.updateAvailable
        ? t("cli.upgrade", { version: info.embeddedCliVersion })
        : t("cli.reinstall");

  return (
    <Card>
      <h2 style={{ marginTop: 0 }}>{t("title")}</h2>
      <p className="muted">{t("cliHint")}</p>

      <div className="row">
        <span data-testid="desktop-cli-state">
          {cli.installedPath
            ? t("cli.installedAt", {
                path: cli.installedPath,
                version: cli.installedVersion ?? "?",
              })
            : t("cli.notInstalled")}
        </span>
        <button
          data-testid="desktop-cli-install"
          disabled={busy}
          onClick={() => void install(false)}
        >
          {buttonLabel}
        </button>
      </div>
      <p className="muted" data-testid="desktop-cli-embedded">
        {t("cli.embedded", { version: info.embeddedCliVersion })}
      </p>

      <div className="row" data-testid="desktop-settings">
        <label>
          <input
            data-testid="desktop-autoupdate-toggle"
            type="checkbox"
            checked={settings.autoUpdateCheck}
            disabled={busy}
            onChange={() => void toggleSetting("autoUpdateCheck")}
          />{" "}
          {t("settings.autoUpdateCheck")}
        </label>
        <label>
          <input
            data-testid="desktop-autostart-toggle"
            type="checkbox"
            checked={settings.autoStart}
            disabled={busy}
            onChange={() => void toggleSetting("autoStart")}
          />{" "}
          {t("settings.autoStart")}
        </label>
        <label>
          <input
            data-testid="desktop-tray-toggle"
            type="checkbox"
            checked={settings.trayResident}
            disabled={busy}
            onChange={() => void toggleSetting("trayResident")}
          />{" "}
          {t("settings.trayResident")}
        </label>
      </div>

      {conflict?.conflict === "external-binary" && (
        <div data-testid="desktop-cli-external" className="notice">
          <p>{t("cli.externalConflict", { path: conflict.installedPath ?? "" })}</p>
          <button
            data-testid="desktop-cli-overwrite"
            disabled={busy}
            onClick={() => void install(true)}
          >
            {t("cli.overwrite")}
          </button>
        </div>
      )}
      {conflict?.conflict === "permission" && (
        <p data-testid="desktop-cli-permission" className="error">
          {t("cli.permissionHint", { hint: conflict.hint ?? "" })}
        </p>
      )}
      {notice && (
        <p data-testid="desktop-cli-notice" className="notice">
          {notice}
        </p>
      )}
      {error && (
        <p data-testid="desktop-cli-error" className="error">
          {error}
        </p>
      )}
    </Card>
  );
}
