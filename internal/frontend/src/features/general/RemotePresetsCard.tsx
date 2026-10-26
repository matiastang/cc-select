import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { RefreshCw } from "lucide-react";
import { Badge, Button, Card, Spinner, Switch, useToast } from "../../components/ui";
import { API_BASE } from "../../constants";

interface PresetsMeta {
  source: "builtin" | "cache" | "remote";
  fetchedAt: string | null;
}

// 远端预设卡（US5 / FR-012–014）：开关 + 手动刷新 + 来源徽标。
// 安全语义见 contracts/api.md：关闭后后端零网络请求。
export function RemotePresetsCard() {
  const { t } = useTranslation("general");
  const { toast } = useToast();
  const [enabled, setEnabled] = useState(true);
  const [meta, setMeta] = useState<PresetsMeta>({ source: "builtin", fetchedAt: null });
  const [warning, setWarning] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const r = await fetch(`${API_BASE}/remote-presets`);
        const body = (await r.json()) as { enabled: boolean };
        if (!cancelled) setEnabled(body.enabled);
      } catch {
        /* 离线：保持默认展示 */
      }
      // 徽标初始真值来自 /presets 的 meta（M9：与画廊徽标一致，不再硬编码）。
      try {
        const r = await fetch(`${API_BASE}/presets`);
        const body = (await r.json()) as { meta?: PresetsMeta };
        if (!cancelled && body.meta) setMeta(body.meta);
      } catch {
        /* 同上 */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const toggle = async (next: boolean) => {
    setEnabled(next);
    try {
      const r = await fetch(`${API_BASE}/remote-presets`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled: next }),
      });
      if (!r.ok) throw new Error(String(r.status));
      toast({
        kind: "success",
        message: next ? t("remotePresets.enabledOn") : t("remotePresets.enabledOff"),
      });
    } catch {
      setEnabled(!next);
      toast({ kind: "error", message: t("remotePresets.toggleFailed") });
    }
  };

  const refresh = async () => {
    setBusy(true);
    setWarning("");
    try {
      const r = await fetch(`${API_BASE}/presets/refresh`, { method: "POST" });
      const body = (await r.json()) as { meta: PresetsMeta; warning?: string };
      setMeta(body.meta);
      if (body.warning) {
        setWarning(body.warning);
        toast({ kind: "info", message: t(`remotePresets.warning.${body.warning}`) });
      } else {
        toast({ kind: "success", message: t("remotePresets.refreshed") });
      }
    } catch {
      setWarning("fetch_failed");
      toast({ kind: "error", message: t("remotePresets.refreshFailed") });
    } finally {
      setBusy(false);
    }
  };

  const sourceLabel = t(`remotePresets.source.${meta.source}`);

  return (
    <Card data-testid="remote-presets-card">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="mb-1 text-base font-semibold">{t("remotePresets.title")}</h2>
          <p className="text-sm text-muted">{t("remotePresets.helper")}</p>
        </div>
        <div className="flex items-center gap-2">
          <Badge
            variant={meta.source === "remote" ? "success" : "muted"}
            data-testid="presets-source-badge"
          >
            {sourceLabel}
          </Badge>
          <Switch
            data-testid="remote-presets-toggle"
            checked={enabled}
            onCheckedChange={(v) => void toggle(v)}
            aria-label={t("remotePresets.enabled")}
          />
        </div>
      </div>
      <div className="mt-3 flex items-center gap-3">
        <Button
          variant="secondary"
          size="sm"
          data-testid="presets-refresh-button"
          disabled={busy}
          onClick={() => void refresh()}
        >
          {busy ? (
            <Spinner label={t("remotePresets.refreshing")} size={14} />
          ) : (
            <RefreshCw size={14} />
          )}
          {t("remotePresets.refresh")}
        </Button>
        <span className="text-xs text-muted">
          {meta.fetchedAt
            ? t("remotePresets.fetchedAt", { time: new Date(meta.fetchedAt).toLocaleString() })
            : t("remotePresets.neverFetched")}
        </span>
      </div>
      {warning && (
        <p data-testid="presets-refresh-warning" className="mt-2 text-xs text-danger">
          {t(`remotePresets.warning.${warning}`)}
        </p>
      )}
    </Card>
  );
}
