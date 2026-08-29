import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button, Card } from "./ui";

type RouteEntry = {
  tid: string;
  provider: string;
  updatedAt: string;
};

type RoutesState = {
  router: { running: boolean; addr: string; version: string };
  routes: RouteEntry[];
};

// RoutesPanel 是 Mode P 的「活跃路由」面板：daemon 存活徽标、路由表（tid 短码）、
// ensure/prune 操作。只读排障视图 + 两个幂等操作（specs/001 contracts/web-api.md §2–4）。
export function RoutesPanel() {
  const { t } = useTranslation("providers");
  const [state, setState] = useState<RoutesState | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const r = await fetch("./api/v1/routes");
      if (!r.ok) throw new Error(String(r.status));
      setState((await r.json()) as RoutesState);
      setError("");
    } catch (e) {
      setError(String(e));
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const ensure = async () => {
    setBusy(true);
    try {
      const r = await fetch("./api/v1/router/ensure", { method: "POST" });
      if (!r.ok) {
        const j = (await r.json().catch(() => ({}))) as { error?: string };
        setError(j.error || t("routes.ensureFailed"));
      }
      await load();
    } finally {
      setBusy(false);
    }
  };

  const prune = async () => {
    setBusy(true);
    try {
      const r = await fetch("./api/v1/routes", { method: "DELETE" });
      if (!r.ok) throw new Error(String(r.status));
      await load();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card>
      <h2 style={{ marginTop: 0 }}>{t("routes.title")}</h2>
      <p className="muted">{t("routes.hint")}</p>
      <div data-testid="routes-panel">
        <div className="row">
          <span
            data-testid="router-badge"
            className={state?.router.running ? "badge badge--ok" : "badge"}
          >
            {state?.router.running
              ? t("routes.routerRunning", { addr: state.router.addr })
              : t("routes.routerDown")}
          </span>
          <Button data-testid="router-ensure-button" disabled={busy} onClick={() => void ensure()}>
            {t("routes.ensureButton")}
          </Button>
          <Button
            disabled={busy || !state || state.routes.length === 0}
            onClick={() => void prune()}
          >
            {t("routes.pruneButton")}
          </Button>
        </div>
        {error && (
          <div className="notice notice--danger" role="alert">
            {error}
          </div>
        )}
        <table>
          <thead>
            <tr>
              <th>TID</th>
              <th>{t("routes.provider")}</th>
              <th>{t("routes.updatedAt")}</th>
            </tr>
          </thead>
          <tbody>
            {(state?.routes ?? []).map((r) => (
              <tr key={r.tid}>
                <td>
                  <code>{r.tid}</code>
                </td>
                <td>{r.provider}</td>
                <td>{new Date(r.updatedAt).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}
