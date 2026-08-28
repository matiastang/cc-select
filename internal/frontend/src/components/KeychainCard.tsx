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

  useEffect(() => {
    void (async () => {
      try {
        const r = await fetch("./api/v1/keychain");
        const j = (await r.json()) as { enabled?: boolean };
        setEnabled(!!j.enabled);
      } catch {
        /* 读不到按默认关闭展示 */
      }
    })();
  }, []);

  const toggle = async () => {
    setBusy(true);
    try {
      const r = await fetch("./api/v1/keychain", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled: !enabled }),
      });
      const j = (await r.json()) as { enabled?: boolean; migrated?: number };
      setEnabled(!!j.enabled);
      if (j.enabled) {
        setNotice(t("keychain.migrated", { n: j.migrated ?? 0 }));
      } else {
        setNotice("");
      }
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
    </Card>
  );
}
