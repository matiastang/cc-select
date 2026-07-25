import { useCallback, useEffect, useState } from "react";
import { Trans, useTranslation } from "react-i18next";

import { IdPlaceholder } from "./components/IdPlaceholder";
import { Button, Textarea } from "./components/ui";

// ShellIntegrationBanner: detects whether shell integration is installed on first visit;
// if not, offers one-click install.
// Self-contained state machine; platform/shell judgement is all on the backend — frontend
// just renders according to Status/InstallResult.
// - zsh / bash (single profile): one "one-click install" button.
// - PowerShell on Windows: PS7 and PS5.1 have SEPARATE profiles — render one button per
//   present variant; grey out the installed ones; hide the whole module once all are installed.
// Already fully installed or unsupported shell (fish, etc.) → hidden, zero interference.
type Target = { id: string; rcPath: string; installed: boolean };

type Phase = "loading" | "needed" | "installing" | "done" | "manual" | "unsupported" | "hidden";

type StatusResp = {
  supported?: boolean;
  shell?: string;
  installed?: boolean;
  legacy?: boolean;
  targets?: Target[];
};

export function ShellIntegrationBanner() {
  const { t } = useTranslation("shell");
  const [phase, setPhase] = useState<Phase>("loading");
  const [shell, setShell] = useState("");
  const [legacy, setLegacy] = useState(false);
  const [targets, setTargets] = useState<Target[]>([]);
  const [installingId, setInstallingId] = useState<string | null>(null);
  const [doneMsg, setDoneMsg] = useState("");
  const [manual, setManual] = useState<{ snippet: string; message: string } | null>(null);
  const [err, setErr] = useState("");

  const fetchStatus = useCallback(async (): Promise<StatusResp> => {
    const r = await fetch("/api/v1/shell-integration");
    if (!r.ok) throw new Error("status");
    return r.json();
  }, []);

  // 由 status 计算targets 列表；空 targets（旧后端/无变体）退化为单按钮流程。
  const toTargets = (d: StatusResp): Target[] => (Array.isArray(d.targets) ? d.targets : []);

  // 全部 target 已装 → 隐藏；否则保持 needed。
  const recompute = (tgts: Target[], topLevelInstalled: boolean) => {
    if (tgts.length > 0) return tgts.every((x) => x.installed) ? "hidden" : "needed";
    return topLevelInstalled ? "hidden" : "needed";
  };

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const d = await fetchStatus();
        if (cancelled) return;
        setShell(d.shell || "");
        setLegacy(d.legacy === true);
        const tgts = toTargets(d);
        setTargets(tgts);
        if (!d.supported) {
          setPhase("unsupported");
        } else {
          setPhase(recompute(tgts, d.installed === true));
        }
      } catch {
        setPhase("hidden"); // fail silently, do not block the main config page.
      }
    })();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const install = async (targetId: string) => {
    setPhase("installing");
    setInstallingId(targetId);
    setErr("");
    try {
      const r = await fetch("/api/v1/shell-integration/install", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ shell, target: targetId }),
      });
      const d = await r.json().catch(() => ({}));
      if (!r.ok) {
        setErr(d.error || t("installFailed"));
        setPhase("needed");
        setInstallingId(null);
        return;
      }
      if (d.action === "manual") {
        setManual({ snippet: d.snippet || "", message: d.message || "" });
        setPhase("manual");
        setInstallingId(null);
        return;
      }
      // 单目标（含 0 目标回退）：维持现有 done ✅ 体验（与旧测试一致）。
      if (targets.length <= 1) {
        setDoneMsg(d.message || t("installed"));
        setPhase("done");
        setInstallingId(null);
        return;
      }
      // 多目标：刷新各 target 状态——装好的置灰，全装完则隐藏。
      const s = await fetchStatus();
      const tgts = toTargets(s);
      setTargets(tgts);
      setInstallingId(null);
      setPhase(recompute(tgts, s.installed === true));
    } catch (e) {
      setErr(String(e));
      setPhase("needed");
      setInstallingId(null);
    }
  };

  if (phase === "loading" || phase === "hidden") return null;

  if (phase === "done") {
    return <div className="notice">✅ {doneMsg}</div>;
  }

  if (phase === "manual") {
    return (
      <div className="notice">
        <strong>{t("manualTitle")}</strong>
        <div className="muted">{manual?.message}</div>
        <Textarea
          readOnly
          value={manual?.snippet || ""}
          spellCheck={false}
          rows={10}
          style={{
            fontFamily: "monospace",
            fontSize: "0.85rem",
            marginTop: "0.5rem",
          }}
        />
      </div>
    );
  }

  if (phase === "unsupported") {
    return (
      <div className="notice">
        <strong>{t("unsupportedTitle", { shell: shell ? `（${shell}）` : "" })}</strong>
        <div className="muted">
          <Trans i18nKey="unsupportedHint" ns="shell" components={{ code: <code /> }} />
        </div>
      </div>
    );
  }

  const errBox = err && (
    <div className="muted" role="alert" style={{ color: "var(--danger)", marginTop: "0.5rem" }}>
      {err}
    </div>
  );

  // 多目标（PowerShell 5.1 + 7 并存等）：每个变体一个按钮，已装的置灰，说明文案区分。
  if (targets.length > 1) {
    return (
      <div className="notice">
        <strong>{t("multiTitle")}</strong>
        <div className="muted">
          <Trans i18nKey="multiHint" ns="shell" components={{ code: <code /> }} />
        </div>
        {errBox}
        <div style={{ marginTop: "0.5rem", display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
          {targets.map((tg) => {
            const busy = installingId === tg.id;
            return (
              <Button
                key={tg.id}
                data-testid={`shell-install-button-${tg.id}`}
                onClick={() => install(tg.id)}
                disabled={tg.installed || busy}
              >
                {tg.installed
                  ? `${t(`target.${tg.id}`)} · ${t("installed")}`
                  : busy
                    ? t("installing")
                    : t(`target.${tg.id}`)}
              </Button>
            );
          })}
        </div>
      </div>
    );
  }

  // 单目标 / 0 目标回退：现有 UI（保持「一键安装」单按钮体验与旧测试）。
  return (
    <div className="notice">
      <strong>
        {t(legacy ? "legacyTitle" : "neededTitle", { shell: shell ? `（${shell}）` : "" })}
      </strong>
      <div className="muted">
        <Trans
          i18nKey="neededHint"
          ns="shell"
          components={{ code: <code />, id: <IdPlaceholder /> }}
        />
      </div>
      {errBox}
      <div style={{ marginTop: "0.5rem" }}>
        <Button
          data-testid="shell-install-button"
          onClick={() => install(targets[0]?.id ?? "")}
          disabled={phase === "installing"}
        >
          {phase === "installing" ? t("installing") : t("installButton")}
        </Button>
      </div>
    </div>
  );
}
