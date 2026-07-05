import { useTranslation } from "react-i18next";
import { Provider } from "../types";
import { OFFICIAL_ID } from "../constants";

type ProviderCardProps = {
  provider: Provider;
  onEdit: (id: string) => void;
  onDelete: (id: string) => void;
};

export function ProviderCard({ provider, onEdit, onDelete }: ProviderCardProps) {
  const { t } = useTranslation("providers");

  const handleDelete = () => {
    if (confirm(t("deleteConfirm", { id: provider.id }))) {
      onDelete(provider.id);
    }
  };

  const modeLabel = provider.isolationMode
    ? t(provider.isolationMode === "settings-only" ? "mode.settingsOnly" : "mode.full")
    : t("mode.inherit");

  return (
    <div className="row" data-testid={`provider-card-${provider.id}`}>
      <div className="row-content">
        <strong>{provider.name || provider.id}</strong>{" "}
        <span className="muted">({provider.id})</span>{" "}
        {provider.hasKey && <span className="badge">{t("configuredKey")}</span>}
        <div className="muted row-detail">
          {provider.id === OFFICIAL_ID ? (
            t("officialNotice")
          ) : (
            <>
              {(provider.env.ANTHROPIC_BASE_URL && t("urlLabel", { url: provider.env.ANTHROPIC_BASE_URL })) || t("noBaseUrl")}
              {provider.env.ANTHROPIC_MODEL && t("modelLabel", { model: provider.env.ANTHROPIC_MODEL })}
              {t("modeLabel", { mode: modeLabel })}
            </>
          )}
        </div>
      </div>
      <div className="row-actions">
        {provider.id !== OFFICIAL_ID && (
          <>
            <button
              data-testid={`edit-provider-${provider.id}`}
              className="secondary"
              onClick={() => onEdit(provider.id)}
            >
              {t("edit", { ns: "common" })}
            </button>{" "}
            <button
              data-testid={`delete-provider-${provider.id}`}
              className="danger"
              onClick={handleDelete}
            >
              {t("delete", { ns: "common" })}
            </button>
          </>
        )}
      </div>
    </div>
  );
}
