import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Card } from "./ui";

// KeychainCard 是钥匙串开关（产品决策 2026-08-29）：默认关闭——保存保持明文原样；
// 用户显式开启后，存量密钥迁入系统钥匙串，且后续保存自动占位化。
export function KeychainCard() {
  const { t } = useTranslation("providers");
  const [enabled, setEnabled] = useState(false);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    void (async () => {
      try {
        const r = await fetch("./api/v1/keychain");
        if (!r.ok) throw new Error(String(r.status));
        const j = (await r.json()) as { enabled?: boolean };
        setEnabled(!!j.enabled);
      } catch {
        setError(t("keychain.loadFailed"));
      }
    })();
  }, [t]);

  const toggle = async () => {
    setBusy(true);
    setError("");
    try {
      const r = await fetch("./api/v1/keychain", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled: !enabled }),
      });
      if (!r.ok) {
        // 失败保持原状态，不误置复选框（评审 P2）。
        setError(t("keychain.saveFailed"));
        return;
      }
      const j = (await r.json()) as { enabled?: boolean; migrated?: number };
      setEnabled(!!j.enabled);
      if (j.enabled) {
        setNotice(t("keychain.migrated", { n: j.migrated ?? 0 }));
      } else {
        setNotice("");
      }
    } catch {
      setError(t("keychain.saveFailed"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card>
      <h2 style={{ marginTop: 0 }}>{t("keychain.title")}</h2>
      <p className="muted">{t("keychain.hint")}</p>
      <div className="row">
        <label>
          <input
            type="checkbox"
            data-testid="keychain-toggle"
            checked={enabled}
            disabled={busy}
            onChange={() => void toggle()}
          />{" "}
          {t("keychain.enable")}
        </label>
      </div>
      {notice && (
        <div className="notice" data-testid="keychain-notice">
          {notice}
        </div>
      )}
      {error && (
        <div className="notice notice--danger" role="alert" data-testid="keychain-error">
          {error}
        </div>
      )}
    </Card>
  );
}
