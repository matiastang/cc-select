import { useTranslation } from "react-i18next";
import { IsolationMode } from "../types";

type GlobalModeCardProps = {
  mode: IsolationMode;
  loading: boolean;
  onChange: (mode: IsolationMode) => void;
};

export function GlobalModeCard({ mode, loading, onChange }: GlobalModeCardProps) {
  const { t } = useTranslation("providers");

  return (
    <div className="card">
      <h2 style={{ marginTop: 0 }}>{t("globalModeTitle")}</h2>
      <p className="muted">{t("globalModeHint")}</p>
      {loading ? (
        <p className="muted">{t("loading", { ns: "common" })}</p>
      ) : (
        <select
          data-testid="global-mode-select"
          value={mode}
          onChange={(e) => onChange(e.target.value as IsolationMode)}
          style={{ width: "100%", padding: "0.5rem", fontSize: "0.95rem" }}
        >
          <option value="settings-only">{t("mode.settingsOnly")}</option>
          <option value="full">{t("mode.full")}</option>
        </select>
      )}
    </div>
  );
}
