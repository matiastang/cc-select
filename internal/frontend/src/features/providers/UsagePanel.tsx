import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Gauge, RefreshCw } from "lucide-react";
import { Button, Progress, Spinner } from "../../components/ui";
import { API_BASE } from "../../constants";

interface UsageWindow {
  type: string;
  used?: number;
  limit?: number;
  percent?: number;
  resetAt?: string;
}

interface UsageReport {
  supported: boolean;
  windows?: UsageWindow[];
  fetchedAt?: string;
  reason?: string;
  error?: string;
}

// 用量面板（US7 / R11 / FR-020）：provider 卡片内嵌，仅手动刷新（FR-019）。
// 不支持 → 占位；失败 → 错误 + 重试；成功 → 窗口进度条 + 重置时间 + fetchedAt。
export function UsagePanel({ providerId }: { providerId: string }) {
  const { t } = useTranslation("providers");
  const [busy, setBusy] = useState(false);
  const [report, setReport] = useState<UsageReport | null>(null);

  const refresh = async () => {
    setBusy(true);
    try {
      const r = await fetch(`${API_BASE}/usage/${encodeURIComponent(providerId)}`, {
        method: "POST",
      });
      setReport((await r.json()) as UsageReport);
    } catch {
      setReport({ supported: false, error: "network" });
    } finally {
      setBusy(false);
    }
  };

  const label = (type: string) => t(`usage.type.${type}`, type);

  return (
    <div data-testid="usage-panel" className="mt-2 flex flex-col gap-2 border-t border-border pt-2">
      <div className="flex items-center gap-2">
        <Gauge size={14} className="text-muted" />
        <span className="text-xs font-medium text-muted">{t("usage.title")}</span>
        <Button
          variant="ghost"
          size="sm"
          data-testid="usage-refresh-button"
          disabled={busy}
          onClick={() => void refresh()}
        >
          {busy ? <Spinner label={t("usage.refreshing")} size={14} /> : <RefreshCw size={14} />}
          {t("usage.refresh")}
        </Button>
        {report?.fetchedAt && (
          <span data-testid="usage-fetched-at" className="text-xs text-muted">
            {t("usage.fetchedAt", { time: new Date(report.fetchedAt).toLocaleTimeString() })}
          </span>
        )}
      </div>

      {report && !report.supported && !report.error && (
        <p data-testid="usage-unsupported" className="text-xs text-muted">
          {t("usage.unsupported")}
        </p>
      )}
      {report?.error && (
        <p data-testid="usage-error" className="text-xs text-danger">
          {t("usage.error", { message: report.error })}
        </p>
      )}
      {report?.supported && report.windows && (
        <div data-testid="usage-windows" className="flex flex-col gap-2">
          {report.windows.map((w, i) => {
            const percent =
              w.percent ??
              (w.limit && w.limit > 0 && w.used !== undefined
                ? (w.used / w.limit) * 100
                : undefined);
            return (
              <div
                key={`${w.type}-${i}`}
                data-testid={`usage-window-${w.type}`}
                className="flex flex-col gap-0.5"
              >
                <div className="flex items-center justify-between text-xs">
                  <span className="font-medium text-text">{label(w.type)}</span>
                  <span className="text-muted">
                    {percent !== undefined
                      ? t("usage.percentValue", { value: Math.round(percent * 10) / 10 })
                      : t("usage.value", { used: w.used ?? 0, limit: w.limit ?? 0 })}
                  </span>
                </div>
                {percent !== undefined && <Progress value={percent} aria-label={label(w.type)} />}
                {w.resetAt && (
                  <span className="text-[11px] text-muted">
                    {t("usage.resetAt", { time: new Date(w.resetAt).toLocaleTimeString() })}
                  </span>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
