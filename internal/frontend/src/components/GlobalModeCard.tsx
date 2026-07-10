import { useId } from "react";
import { useTranslation } from "react-i18next";
import { IsolationMode } from "../types";
import { Card, FormField, Select } from "./ui";

type GlobalModeCardProps = {
  mode: IsolationMode;
  loading: boolean;
  onChange: (mode: IsolationMode) => void;
};

export function GlobalModeCard({ mode, loading, onChange }: GlobalModeCardProps) {
  const { t } = useTranslation("providers");
  const controlId = useId();

  return (
    <Card>
      <h2 style={{ marginTop: 0 }}>{t("globalModeTitle")}</h2>
      <p className="muted">{t("globalModeHint")}</p>
      {loading ? (
        <p className="muted">{t("loading", { ns: "common" })}</p>
      ) : (
        <FormField label={t("globalModeTitle")} htmlFor={controlId}>
          <Select
            id={controlId}
            data-testid="global-mode-select"
            value={mode}
            onChange={(e) => onChange(e.target.value as IsolationMode)}
            aria-label={t("globalModeTitle")}
          >
            <option value="settings-only">{t("mode.settingsOnly")}</option>
            <option value="full">{t("mode.full")}</option>
          </Select>
        </FormField>
      )}
    </Card>
  );
}
