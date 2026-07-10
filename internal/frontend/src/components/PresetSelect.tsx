import { useId } from "react";
import { useTranslation } from "react-i18next";
import { Preset, PresetCategory, groupPresetsByCategory } from "../presets/presets";
import { FormField, Select, Icon } from "./ui";

type PresetSelectProps = {
  presets: Preset[];
  categories: PresetCategory[];
  value: string;
  onChange: (id: string) => void;
  label: string;
};

export function PresetSelect({ presets, categories, value, onChange, label }: PresetSelectProps) {
  const { t } = useTranslation("providers");
  const groups = groupPresetsByCategory(presets, categories);
  const selectId = useId();

  return (
    <FormField label={label} htmlFor={selectId}>
      <Select
        id={selectId}
        data-testid="preset-select"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      >
        {categories.map((cat) => (
          <optgroup key={cat} label={t(`presetCategory.${cat}`)}>
            {groups[cat]?.map((p) => (
              <option key={p.id} value={p.id}>
                {p.displayName}
              </option>
            ))}
          </optgroup>
        ))}
      </Select>
      <PresetLinks preset={presets.find((p) => p.id === value)} t={t} />
    </FormField>
  );
}

function PresetLinks({ preset, t }: { preset?: Preset; t: (key: string) => string }) {
  if (!preset) return null;
  return (
    <div
      style={{
        marginTop: "0.35rem",
        fontSize: "0.8rem",
        display: "flex",
        gap: "0.75rem",
        alignItems: "center",
      }}
    >
      {preset.websiteURL && (
        <a
          href={preset.websiteURL}
          target="_blank"
          rel="noopener noreferrer"
          style={{ display: "inline-flex", alignItems: "center", gap: "0.25rem" }}
        >
          <Icon name="globe" size={14} />
          {t("form.websiteLabel")}
        </a>
      )}
      {preset.apiKeyURL && (
        <a
          href={preset.apiKeyURL}
          target="_blank"
          rel="noopener noreferrer"
          style={{ display: "inline-flex", alignItems: "center", gap: "0.25rem" }}
        >
          <Icon name="key" size={14} />
          {t("form.getApiKeyLabel")}
        </a>
      )}
    </div>
  );
}
