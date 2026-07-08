import { useTranslation } from "react-i18next";
import { Preset, PresetCategory, groupPresetsByCategory, CUSTOM_PRESET_ID } from "../presets/presets";

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

  return (
    <div style={{ marginBottom: "1rem" }}>
      <label style={{ display: "block", marginBottom: "0.25rem" }}>{label}</label>
      <select
        data-testid="preset-select"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        style={{ width: "100%", padding: "0.5rem", fontSize: "0.95rem" }}
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
      </select>
      <PresetLinks preset={presets.find((p) => p.id === value)} t={t} />
    </div>
  );
}

function PresetLinks({ preset, t }: { preset?: Preset; t: (key: string) => string }) {
  if (!preset) return null;
  return (
    <div style={{ marginTop: "0.25rem", fontSize: "0.8rem" }}>
      {preset.websiteURL && (
        <a href={preset.websiteURL} target="_blank" rel="noreferrer">
          {t("form.websiteLabel")}
        </a>
      )}
      {preset.websiteURL && preset.apiKeyURL && " · "}
      {preset.apiKeyURL && (
        <a href={preset.apiKeyURL} target="_blank" rel="noreferrer">
          {t("form.getApiKeyLabel")}
        </a>
      )}
    </div>
  );
}

export { CUSTOM_PRESET_ID };
