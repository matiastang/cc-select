import { useEffect, useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import { Button, Card, useToast } from "../../components/ui";
import { ShellIntegrationBanner } from "./ShellIntegrationBanner";
import { IdPlaceholder } from "../../components/IdPlaceholder";
import { ProviderList } from "./ProviderList";
import { JsonForm } from "./JsonForm";
import { PresetGallery } from "./PresetGallery";
import { CUSTOM_PRESET_ID } from "../../presets/presets";
import { Provider } from "../../types";
import { API_BASE } from "../../constants";

const API = `${API_BASE}/providers`;

// 「服务商」模块（首页，高频）：shell 集成横幅 + 列表 + 添加入口。
// T008/T009 将把添加入口演进为画廊 + 向导；本组件是零回归中转态。
export function ProvidersModule() {
  const { t } = useTranslation("providers");
  const { toast } = useToast();
  const [providers, setProviders] = useState<Record<string, Provider>>({});
  const [editing, setEditing] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [galleryOpen, setGalleryOpen] = useState(false);
  const [initialPreset, setInitialPreset] = useState<string | undefined>(undefined);
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
    if (!r.ok) {
      toast({
        kind: "error",
        message: (await r.json().catch(() => ({}))).error || t("errors.deleteFailed"),
      });
    } else {
      toast({ kind: "success", message: t("toast.deleted") });
    }
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
          toast({ kind: "success", message: t("toast.saved") });
          refresh();
        }}
        onAdd={() => setGalleryOpen(true)}
      />

      {!creating && (
        <Card>
          <div className="row">
            <Button data-testid="add-provider-button" onClick={() => setGalleryOpen(true)}>
              {t("addButton")}
            </Button>
          </div>
        </Card>
      )}

      {creating && (
        <Card>
          <JsonForm
            mode="create"
            initialPresetId={initialPreset}
            onCancel={() => {
              setCreating(false);
              setInitialPreset(undefined);
            }}
            onSaved={() => {
              setCreating(false);
              setInitialPreset(undefined);
              toast({ kind: "success", message: t("toast.saved") });
              refresh();
            }}
          />
        </Card>
      )}

      <PresetGallery
        open={galleryOpen}
        onClose={() => setGalleryOpen(false)}
        onPick={(presetId) => {
          setGalleryOpen(false);
          setInitialPreset(presetId === CUSTOM_PRESET_ID ? undefined : presetId);
          setCreating(true);
        }}
      />
    </div>
  );
}
