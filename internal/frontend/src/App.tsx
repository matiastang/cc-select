import { useEffect, useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import { ShellIntegrationBanner } from "./ShellIntegrationBanner";
import { Header } from "./components/Header";
import { GlobalModeCard } from "./components/GlobalModeCard";
import { ProviderList } from "./components/ProviderList";
import { JsonForm } from "./components/JsonForm";
import { Provider, IsolationMode } from "./types";

import { API_BASE } from "./constants";

const API = `${API_BASE}/providers`;
const MODE_API = `${API_BASE}/mode`;

export default function App() {
  const { t } = useTranslation("providers");
  const [providers, setProviders] = useState<Record<string, Provider>>({});
  const [globalMode, setGlobalMode] = useState<IsolationMode>("settings-only");
  const [globalModeLoading, setGlobalModeLoading] = useState(true);
  const [editing, setEditing] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string>("");

  const refresh = async () => {
    try {
      const r = await fetch(API);
      const data = await r.json();
      setProviders(data.providers || {});
      setError("");
    } catch (e) {
      setError(String(e));
    }
  };

  const loadGlobalMode = async () => {
    try {
      const r = await fetch(MODE_API);
      const data = await r.json();
      setGlobalMode(data.isolationMode || "settings-only");
    } catch (e) {
      setError(String(e));
    } finally {
      setGlobalModeLoading(false);
    }
  };

  useEffect(() => {
    refresh();
    loadGlobalMode();
  }, []);

  const saveGlobalMode = async (mode: IsolationMode) => {
    setGlobalMode(mode);
    try {
      const r = await fetch(MODE_API, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ isolationMode: mode }),
      });
      if (!r.ok) {
        const j = await r.json().catch(() => ({}));
        setError(j.error || t("errors.saveGlobalModeFailed"));
        await loadGlobalMode(); // rollback to server value
      } else {
        setError("");
      }
    } catch (e) {
      setError(String(e));
      await loadGlobalMode();
    }
  };

  const remove = async (id: string) => {
    const r = await fetch(`${API}/${id}`, { method: "DELETE" });
    if (!r.ok) setError((await r.json().catch(() => ({}))).error || t("errors.deleteFailed"));
    refresh();
  };

  return (
    <div className="container">
      <Header />

      <ShellIntegrationBanner />

      <div className="notice">
        <Trans i18nKey="notice" ns="common">
          配置以完整 <code>settings.json</code> 形式编辑（不止 <code>env</code>，<code>permissions</code>、<code>model</code> 等均可）。在此处修改是改“模板”，<strong>已在运行的终端不会自动变化</strong>，需在对应终端重新执行 <code>ccs use <span>&lt;id&gt;</span></code> 才生效。
        </Trans>
      </div>

      {error && (
        <div
          className="notice"
          style={{ background: "rgba(209,36,47,0.1)", borderLeftColor: "var(--danger)" }}
        >
          {error}
        </div>
      )}

      <GlobalModeCard
        mode={globalMode}
        loading={globalModeLoading}
        onChange={saveGlobalMode}
      />

      <ProviderList
        providers={Object.values(providers)}
        editingId={editing}
        onEditStart={setEditing}
        onEditCancel={() => setEditing(null)}
        onDelete={remove}
        onSaved={() => {
          setEditing(null);
          refresh();
        }}
      />

      <div className="card">
        {creating ? (
          <JsonForm
            mode="create"
            onCancel={() => setCreating(false)}
            onSaved={() => {
              setCreating(false);
              refresh();
            }}
          />
        ) : (
          <div className="row">
            <button data-testid="add-provider-button" onClick={() => setCreating(true)}>{t("addButton")}</button>
          </div>
        )}
      </div>
    </div>
  );
}
