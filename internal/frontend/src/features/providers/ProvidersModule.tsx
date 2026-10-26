import { useEffect, useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import { Button, Card } from "../../components/ui";
import { ShellIntegrationBanner } from "../../ShellIntegrationBanner";
import { IdPlaceholder } from "../../components/IdPlaceholder";
import { ProviderList } from "../../components/ProviderList";
import { JsonForm } from "../../components/JsonForm";
import { Provider } from "../../types";
import { API_BASE } from "../../constants";

const API = `${API_BASE}/providers`;

// 「服务商」模块（首页，高频）：shell 集成横幅 + 列表 + 添加入口。
// T008/T009 将把添加入口演进为画廊 + 向导；本组件是零回归中转态。
export function ProvidersModule() {
  const { t } = useTranslation("providers");
  const [providers, setProviders] = useState<Record<string, Provider>>({});
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

  useEffect(() => {
    void refresh();
  }, []);

  const remove = async (id: string) => {
    const r = await fetch(`${API}/${id}`, { method: "DELETE" });
    if (!r.ok) setError((await r.json().catch(() => ({}))).error || t("errors.deleteFailed"));
    refresh();
  };

  return (
    <div data-testid="module-providers" className="flex flex-col gap-4">
      <p className="text-sm text-muted">
        <Trans i18nKey="subtitle" components={{ code: <code />, id: <IdPlaceholder /> }} />
      </p>
      <ShellIntegrationBanner />

      {error && (
        <div className="notice notice--danger" role="alert">
          {error}
        </div>
      )}

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
