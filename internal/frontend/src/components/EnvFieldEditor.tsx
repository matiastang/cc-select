import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  PresetDetail,
  BASE_URL_KEY,
  MODEL_KEY,
  SONNET_MODEL_KEY,
  OPUS_MODEL_KEY,
  HAIKU_MODEL_KEY,
  FABLE_MODEL_KEY,
  SUBAGENT_MODEL_KEY,
} from "../presets/presets";

type EnvField = {
  key: string;
  labelKey: string;
  placeholderKey?: string;
  type?: "text" | "select";
  options?: string[];
};

export type EnvFieldValues = Record<string, string>;

type EnvFieldEditorProps = {
  preset: PresetDetail | null;
  values: EnvFieldValues;
  onChange: (key: string, value: string) => void;
  apiKey: string;
  onApiKeyChange: (value: string) => void;
  apiFormat: string;
  onApiFormatChange: (value: string) => void;
  authField: string;
  onAuthFieldChange: (value: string) => void;
};

export function EnvFieldEditor({
  preset,
  values,
  onChange,
  apiKey,
  onApiKeyChange,
  apiFormat,
  onApiFormatChange,
  authField,
  onAuthFieldChange,
}: EnvFieldEditorProps) {
  const { t } = useTranslation("providers");
  const [showKey, setShowKey] = useState(false);
  const [showModelMapping, setShowModelMapping] = useState(false);
  const [showCommon, setShowCommon] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);

  const oauth = preset?.oauth ?? false;
  const authFieldName = authField || preset?.authField || "ANTHROPIC_AUTH_TOKEN";

  const commonFields: EnvField[] = [
    { key: BASE_URL_KEY, labelKey: "form.baseURLLabel", placeholderKey: "form.baseURLPlaceholder" },
    { key: MODEL_KEY, labelKey: "form.modelLabel", placeholderKey: "form.modelPlaceholder" },
  ];

  const modelFields: EnvField[] = [
    { key: SONNET_MODEL_KEY, labelKey: "form.sonnetModelLabel", placeholderKey: "form.modelPlaceholder" },
    { key: OPUS_MODEL_KEY, labelKey: "form.opusModelLabel", placeholderKey: "form.modelPlaceholder" },
    { key: HAIKU_MODEL_KEY, labelKey: "form.haikuModelLabel", placeholderKey: "form.modelPlaceholder" },
    { key: FABLE_MODEL_KEY, labelKey: "form.fableModelLabel", placeholderKey: "form.modelPlaceholder" },
    { key: SUBAGENT_MODEL_KEY, labelKey: "form.subagentModelLabel", placeholderKey: "form.modelPlaceholder" },
  ];

  const commonToggleFields: EnvField[] = [
    { key: "CLAUDE_CODE_HIDE_AI_INDICATOR", labelKey: "form.hideAIIndicatorLabel", type: "select", options: ["", "true", "false"] },
    { key: "CLAUDE_CODE_ENABLE_TEAMMATES", labelKey: "form.enableTeammatesLabel", type: "select", options: ["", "true", "false"] },
    { key: "CLAUDE_CODE_ENABLE_TOOL_SEARCH", labelKey: "form.enableToolSearchLabel", type: "select", options: ["", "true", "false"] },
    { key: "CLAUDE_CODE_MAX_THINKING", labelKey: "form.maxThinkingLabel", type: "select", options: ["", "true", "false"] },
    { key: "CLAUDE_CODE_DISABLE_AUTOUPDATE", labelKey: "form.disableAutoUpdateLabel", type: "select", options: ["", "true", "false"] },
  ];

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem" }}>
      {!oauth && (
        <div>
          <label>{t("form.apiKeyLabel", { field: authFieldName })}</label>
          <div style={{ display: "flex", gap: "0.5rem" }}>
            <input
              data-testid="provider-api-key-input"
              type={showKey ? "text" : "password"}
              value={apiKey}
              onChange={(e) => onApiKeyChange(e.target.value)}
              placeholder={t("form.apiKeyPlaceholder")}
              style={{ flex: 1, padding: "0.5rem" }}
            />
            <button
              type="button"
              className="secondary"
              onClick={() => setShowKey((s) => !s)}
              data-testid="provider-api-key-toggle"
              style={{ whiteSpace: "nowrap" }}
            >
              {showKey ? t("form.hideApiKey") : t("form.showApiKey")}
            </button>
          </div>
        </div>
      )}

      {commonFields.map((f) => (
        <TextField key={f.key} field={f} value={values[f.key] || ""} onChange={(v) => onChange(f.key, v)} />
      ))}

      <button
        type="button"
        className="secondary"
        onClick={() => setShowModelMapping((s) => !s)}
        data-testid="toggle-model-mapping"
      >
        {showModelMapping ? "▾" : "▸"} {t("form.modelMappingTitle")}
      </button>
      {showModelMapping && (
        <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
          {modelFields.map((f) => (
            <TextField key={f.key} field={f} value={values[f.key] || ""} onChange={(v) => onChange(f.key, v)} />
          ))}
        </div>
      )}

      <button
        type="button"
        className="secondary"
        onClick={() => setShowCommon((s) => !s)}
        data-testid="toggle-common-settings"
      >
        {showCommon ? "▾" : "▸"} {t("form.commonSettingsTitle")}
      </button>
      {showCommon && (
        <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
          {commonToggleFields.map((f) => (
            <SelectField key={f.key} field={f} value={values[f.key] || ""} onChange={(v) => onChange(f.key, v)} />
          ))}
        </div>
      )}

      <button
        type="button"
        className="secondary"
        onClick={() => setShowAdvanced((s) => !s)}
        data-testid="toggle-advanced"
      >
        {showAdvanced ? "▾" : "▸"} {t("form.advancedTitle")}
      </button>
      {showAdvanced && (
        <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
          <div>
            <label>{t("form.apiFormatLabel")}</label>
            <select
              data-testid="provider-api-format-select"
              value={apiFormat}
              onChange={(e) => onApiFormatChange(e.target.value)}
              style={{ width: "100%", padding: "0.5rem" }}
            >
              <option value="">{t("form.defaultOption", { value: preset?.apiFormat || "anthropic" })}</option>
              <option value="anthropic">anthropic</option>
              <option value="openai_chat">openai_chat</option>
              <option value="openai_responses">openai_responses</option>
              <option value="gemini_native">gemini_native</option>
            </select>
          </div>
          <div>
            <label>{t("form.authFieldLabel")}</label>
            <select
              data-testid="provider-auth-field-select"
              value={authField}
              onChange={(e) => onAuthFieldChange(e.target.value)}
              style={{ width: "100%", padding: "0.5rem" }}
            >
              <option value="">{t("form.defaultOption", { value: preset?.authField || "ANTHROPIC_AUTH_TOKEN" })}</option>
              <option value="ANTHROPIC_AUTH_TOKEN">ANTHROPIC_AUTH_TOKEN</option>
              <option value="ANTHROPIC_API_KEY">ANTHROPIC_API_KEY</option>
            </select>
          </div>
        </div>
      )}
    </div>
  );
}

function TextField({ field, value, onChange }: { field: EnvField; value: string; onChange: (v: string) => void }) {
  const { t } = useTranslation("providers");
  return (
    <div>
      <label>{t(field.labelKey)}</label>
      <input
        data-testid={`env-field-${field.key}`}
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={field.placeholderKey ? t(field.placeholderKey) : undefined}
        style={{ width: "100%", padding: "0.5rem" }}
      />
    </div>
  );
}

function SelectField({ field, value, onChange }: { field: EnvField; value: string; onChange: (v: string) => void }) {
  const { t } = useTranslation("providers");
  return (
    <div>
      <label>{t(field.labelKey)}</label>
      <select
        data-testid={`env-field-${field.key}`}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        style={{ width: "100%", padding: "0.5rem" }}
      >
        {field.options?.map((opt) => (
          <option key={opt} value={opt}>
            {opt === "" ? t("form.unsetOption") : opt}
          </option>
        ))}
      </select>
    </div>
  );
}
