import { useState } from "react";
import { useTranslation } from "react-i18next";
import { PlugZap } from "lucide-react";
import { Button, Spinner } from "../../components/ui";
import { API_BASE } from "../../constants";

interface ProbeResponse {
  ok: boolean;
  latencyMs?: number;
  model?: string;
  error?: { kind: string; message: string };
}

type ProbeButtonProps = {
  // 探测用 env 快照（向导当前表单值；未保存也可测）
  env: () => Record<string, string>;
};

// 「测试连接」（US6 / FR-016-017）：仅手动触发；四态（空闲/进行/成功/失败分类）。
// 必填（认证 + base URL 时的模型）缺失则禁用——与后端 400 前置语义对齐。
export function ProbeButton({ env }: ProbeButtonProps) {
  const { t } = useTranslation("providers");
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<ProbeResponse | null>(null);

  const snapshot = env();
  const hasAuth = !!(snapshot.ANTHROPIC_AUTH_TOKEN || snapshot.ANTHROPIC_API_KEY);
  const needsModel = !!snapshot.ANTHROPIC_BASE_URL && !snapshot.ANTHROPIC_MODEL;
  const disabled = busy || !hasAuth || needsModel;

  const run = async () => {
    setBusy(true);
    setResult(null);
    try {
      const r = await fetch(`${API_BASE}/providers/probe`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ env: snapshot }),
      });
      setResult((await r.json()) as ProbeResponse);
    } catch {
      setResult({ ok: false, error: { kind: "network", message: "" } });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex flex-col gap-1.5" data-testid="probe-button-wrap">
      <Button
        variant="secondary"
        size="sm"
        data-testid="probe-test-button"
        disabled={disabled}
        onClick={() => void run()}
      >
        {busy ? <Spinner label={t("probe.running")} size={14} /> : <PlugZap size={14} />}
        {t("probe.run")}
      </Button>
      {result && (
        <p
          data-testid="probe-result"
          className={result.ok ? "text-xs text-emerald-600" : "text-xs text-danger"}
        >
          {result.ok
            ? t("probe.ok", { model: result.model ?? "", latency: result.latencyMs ?? 0 })
            : t("probe.error", {
                kind: t(`probe.kind.${result.error?.kind ?? "unknown"}`),
                message: result.error?.message ?? "",
              })}
        </p>
      )}
    </div>
  );
}
