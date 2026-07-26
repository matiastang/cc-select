import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "./ui";
import { API_BASE } from "../constants";

// CheckUpdateButton: header-controls 里的自更新入口。
// 自包含状态机（同 ShellIntegrationBanner 的约定）：平台判断全在后端，
// 前端按 check/run 响应渲染；检查失败静默隐藏，不干扰配置页。
// - 挂载时 GET /update/check，有更新显示「立即更新」小按钮；
// - 点击 POST /update，成功后显示「重启生效」卡片（server 进程仍是旧版本，
//   需用户手动重启——见 docs 自更新方案决策 2）；
// - 409 refused（dev/brew/scoop/不可写）显示对应的精确升级指引。
type CheckResp = {
  currentVersion?: string;
  latestVersion?: string;
  hasUpdate?: boolean;
  devBuild?: boolean;
  releaseNotes?: string;
  htmlUrl?: string;
};

type Phase = "idle" | "available" | "installing" | "installed" | "refused" | "error";

// 轮询间隔 6h：单用户本地 GUI 远低于 GitHub 未认证 60/hr 限流。
const CHECK_INTERVAL_MS = 6 * 60 * 60 * 1000;

export function CheckUpdateButton() {
  const { t } = useTranslation("update");
  const [phase, setPhase] = useState<Phase>("idle");
  const [info, setInfo] = useState<CheckResp>({});
  const [refusedKind, setRefusedKind] = useState("");
  const [err, setErr] = useState("");

  const check = useCallback(async () => {
    try {
      const r = await fetch(`${API_BASE}/update/check`);
      if (!r.ok) return;
      const d: CheckResp = await r.json();
      setInfo(d);
      // 只在 idle 时翻转状态，不覆盖 installed/refused 等用户正在看的卡片。
      setPhase((p) => (p === "idle" && d.hasUpdate ? "available" : p));
    } catch {
      // fail silently: 更新检查失败不阻塞主配置页。
    }
  }, []);

  useEffect(() => {
    check();
    const id = setInterval(check, CHECK_INTERVAL_MS);
    return () => clearInterval(id);
  }, [check]);

  const install = async () => {
    setPhase("installing");
    setErr("");
    try {
      const r = await fetch(`${API_BASE}/update`, { method: "POST" });
      const d = await r.json().catch(() => ({}));
      if (!r.ok) {
        if (d.refused) {
          setRefusedKind(typeof d.kind === "string" ? d.kind : "");
          setPhase("refused");
          return;
        }
        setErr(d.error || String(r.status));
        setPhase("error");
        return;
      }
      setInfo((p) => ({ ...p, latestVersion: d.toVersion || p.latestVersion }));
      setPhase("installed");
    } catch (e) {
      setErr(String(e));
      setPhase("error");
    }
  };

  if (phase === "idle") return null;

  // 结果卡片：absolute 浮在按钮下方，复用 .notice 样式（不改全局 CSS）。
  const card = (content: React.ReactNode, testid: string) => (
    <div
      className="notice"
      data-testid={testid}
      style={{
        position: "absolute",
        right: 0,
        top: "calc(100% + 0.5rem)",
        width: "22rem",
        zIndex: 20,
        textAlign: "left",
      }}
    >
      {content}
    </div>
  );

  const refusedMsgKey =
    refusedKind === "homebrew"
      ? "managedHomebrew"
      : refusedKind === "scoop"
        ? "managedScoop"
        : refusedKind === "notWritable"
          ? "notWritable"
          : "devBuild";

  return (
    <span style={{ position: "relative", display: "inline-block" }}>
      {phase === "available" && (
        <Button size="sm" data-testid="update-now-button" onClick={install}>
          {t("installButton")} · v{info.latestVersion}
        </Button>
      )}
      {phase === "available" && info.htmlUrl && (
        <a
          className="muted"
          data-testid="update-changelog-link"
          href={info.htmlUrl}
          target="_blank"
          rel="noreferrer"
          style={{ marginLeft: "0.5rem", fontSize: "0.8rem" }}
        >
          {t("changelog")}
        </a>
      )}
      {phase === "installing" && (
        <Button size="sm" disabled data-testid="update-installing">
          {t("installing")}
        </Button>
      )}
      {(phase === "installed" || phase === "refused" || phase === "error") && (
        <Button
          size="sm"
          variant="secondary"
          data-testid="update-dismiss-button"
          onClick={() => setPhase("idle")}
        >
          {phase === "installed" ? `✓ v${info.latestVersion}` : t("badge")}
        </Button>
      )}
      {phase === "installed" &&
        card(
          <>
            <strong>{t("installed", { version: info.latestVersion })}</strong>
            <div className="muted">{t("restartHint")}</div>
            {info.htmlUrl && (
              <a
                data-testid="update-changelog-link"
                href={info.htmlUrl}
                target="_blank"
                rel="noreferrer"
              >
                {t("changelog")}
              </a>
            )}
          </>,
          "update-installed-card",
        )}
      {phase === "refused" &&
        card(<div className="muted">{t(refusedMsgKey)}</div>, "update-refused-card")}
      {phase === "error" &&
        card(
          <div className="muted" role="alert" style={{ color: "var(--danger)" }}>
            {t("error", { error: err })}
          </div>,
          "update-error-card",
        )}
    </span>
  );
}
