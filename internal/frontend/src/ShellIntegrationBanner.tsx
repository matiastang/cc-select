import { useEffect, useState } from "react";
import { Trans, useTranslation } from "react-i18next";

import { IdPlaceholder } from "./components/IdPlaceholder";
import { Button, Textarea } from "./components/ui";

// ShellIntegrationBanner: detects whether shell integration is installed on first visit;
// if not, offers one-click install.
// Self-contained state machine; platform/shell judgement is all on the backend — frontend
// just renders according to Status/InstallResult.
// Already installed or unsupported shell (fish, etc.) → hidden, zero interference.
export function ShellIntegrationBanner() {
  const { t } = useTranslation("shell");
  const [state, setState] = useState<
    "loading" | "needed" | "installing" | "done" | "manual" | "unsupported" | "hidden"
  >("loading");
  const [shell, setShell] = useState("");
  const [legacy, setLegacy] = useState(false);
  const [doneMsg, setDoneMsg] = useState("");
  const [manual, setManual] = useState<{ snippet: string; message: string } | null>(null);
  const [err, setErr] = useState("");

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const r = await fetch("/api/v1/shell-integration");
        if (!r.ok) return;
        const d = await r.json();
        if (cancelled) return;
        setShell(d.shell || "");
        setLegacy(d.legacy === true);
        if (!d.supported) {
          setState("unsupported");
        } else if (d.installed) {
          setState("hidden");
        } else {
          setState("needed");
        }
      } catch {
        setState("hidden"); // fail silently, do not block the main config page.
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const install = async () => {
    setState("installing");
    setErr("");
    try {
      const r = await fetch("/api/v1/shell-integration/install", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ shell }),
      });
      const d = await r.json().catch(() => ({}));
      if (!r.ok) {
        setErr(d.error || t("installFailed"));
        setState("needed");
        return;
      }
      if (d.action === "manual") {
        setManual({ snippet: d.snippet || "", message: d.message || "" });
        setState("manual");
      } else {
        setDoneMsg(d.message || t("installed"));
        setState("done");
      }
    } catch (e) {
      setErr(String(e));
      setState("needed");
    }
  };

  if (state === "loading" || state === "hidden") return null;

  if (state === "done") {
    return <div className="notice">✅ {doneMsg}</div>;
  }

  if (state === "manual") {
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

  if (state === "unsupported") {
    return (
      <div className="notice">
        <strong>{t("unsupportedTitle", { shell: shell ? `（${shell}）` : "" })}</strong>
        <div className="muted">
          <Trans i18nKey="unsupportedHint" ns="shell" components={{ code: <code /> }} />
        </div>
      </div>
    );
  }

  // needed / installing
  return (
    <div className="notice">
      <strong>
        {t(legacy ? "legacyTitle" : "neededTitle", { shell: shell ? `（${shell}）` : "" })}
      </strong>
      <div className="muted">
        <Trans i18nKey="neededHint" ns="shell" components={{ code: <code />, id: <IdPlaceholder /> }} />
      </div>
      {err && (
        <div className="muted" role="alert" style={{ color: "var(--danger)", marginTop: "0.5rem" }}>
          {err}
        </div>
      )}
      <div style={{ marginTop: "0.5rem" }}>
        <Button data-testid="shell-install-button" onClick={install} disabled={state === "installing"}>
          {state === "installing" ? t("installing") : t("installButton")}
        </Button>
      </div>
    </div>
  );
}
