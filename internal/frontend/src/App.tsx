import { useEffect, useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import i18n from "./i18n";
import { IdPlaceholder } from "./components/IdPlaceholder";
import { ShellIntegrationBanner } from "./ShellIntegrationBanner";
import { Header } from "./components/Header";
import { GlobalModeCard } from "./components/GlobalModeCard";
import { ProviderList } from "./components/ProviderList";
import { JsonForm } from "./components/JsonForm";
import { Provider, IsolationMode } from "./types";
import { fetchBackendLanguage } from "./i18n/useLanguage";
import { Button, Card } from "./components/ui";

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

  const loadLanguage = async () => {
    const backend = await fetchBackendLanguage();
    if (backend) {
      await i18n.changeLanguage(backend);
    }
  };

  useEffect(() => {
    refresh();
    loadGlobalMode();
    loadLanguage();
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
        <Trans
          i18nKey="notice"
          ns="common"
          components={{ code: <code />, strong: <strong />, id: <IdPlaceholder /> }}
        />
      </div>

      {error && (
        <div className="notice notice--danger" role="alert">
          {error}
        </div>
      )}

      <GlobalModeCard mode={globalMode} loading={globalModeLoading} onChange={saveGlobalMode} />

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

      <Card>
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
            <Button data-testid="add-provider-button" onClick={() => setCreating(true)}>
              {t("addButton")}
            </Button>
          </div>
        )}
      </Card>
    </div>
  );
}
