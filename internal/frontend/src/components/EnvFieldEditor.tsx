import { useId, useState } from "react";
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
import { Button, Collapsible, FormField, Input, SegmentedControl, Select } from "./ui";

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
  const apiFormatId = useId();
  const authFieldId = useId();

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
    <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
      {!oauth && (
        <FormField
          label={t("form.apiKeyLabel", { field: authFieldName })}
          htmlFor="provider-api-key-input"
        >
          <div style={{ display: "flex", gap: "0.5rem" }}>
            <Input
              id="provider-api-key-input"
              data-testid="provider-api-key-input"
              type={showKey ? "text" : "password"}
              value={apiKey}
              onChange={(e) => onApiKeyChange(e.target.value)}
              placeholder={t("form.apiKeyPlaceholder")}
              style={{ flex: 1 }}
            />
            <Button
              type="button"
              variant="secondary"
              size="sm"
              icon={showKey ? "eyeOff" : "eye"}
              onClick={() => setShowKey((s) => !s)}
              data-testid="provider-api-key-toggle"
              aria-pressed={showKey}
              aria-label={showKey ? t("form.hideApiKey") : t("form.showApiKey")}
              style={{ whiteSpace: "nowrap" }}
            >
              {showKey ? t("form.hideApiKey") : t("form.showApiKey")}
            </Button>
          </div>
        </FormField>
      )}

      {commonFields.map((f) => (
        <TextField key={f.key} field={f} value={values[f.key] || ""} onChange={(v) => onChange(f.key, v)} />
      ))}

      <Collapsible
        title={t("form.modelMappingTitle")}
        open={showModelMapping}
        onToggle={() => setShowModelMapping((s) => !s)}
        data-testid="toggle-model-mapping"
      >
        {modelFields.map((f) => (
          <TextField key={f.key} field={f} value={values[f.key] || ""} onChange={(v) => onChange(f.key, v)} />
        ))}
      </Collapsible>

      <Collapsible
        title={t("form.commonSettingsTitle")}
        open={showCommon}
        onToggle={() => setShowCommon((s) => !s)}
        data-testid="toggle-common-settings"
      >
        {commonToggleFields.map((f) => (
          <SelectField key={f.key} field={f} value={values[f.key] || ""} onChange={(v) => onChange(f.key, v)} />
        ))}
      </Collapsible>

      <Collapsible
        title={t("form.advancedTitle")}
        open={showAdvanced}
        onToggle={() => setShowAdvanced((s) => !s)}
        data-testid="toggle-advanced"
      >
        <FormField label={t("form.apiFormatLabel")} htmlFor={apiFormatId}>
          <Select
            id={apiFormatId}
            data-testid="provider-api-format-select"
            value={apiFormat}
            onChange={(e) => onApiFormatChange(e.target.value)}
          >
            <option value="">{t("form.defaultOption", { value: preset?.apiFormat || "anthropic" })}</option>
            <option value="anthropic">anthropic</option>
            <option value="openai_chat">openai_chat</option>
            <option value="openai_responses">openai_responses</option>
            <option value="gemini_native">gemini_native</option>
          </Select>
        </FormField>

        <FormField label={t("form.authFieldLabel")} htmlFor={authFieldId}>
          <Select
            id={authFieldId}
            data-testid="provider-auth-field-select"
            value={authField}
            onChange={(e) => onAuthFieldChange(e.target.value)}
          >
            <option value="">{t("form.defaultOption", { value: preset?.authField || "ANTHROPIC_AUTH_TOKEN" })}</option>
            <option value="ANTHROPIC_AUTH_TOKEN">ANTHROPIC_AUTH_TOKEN</option>
            <option value="ANTHROPIC_API_KEY">ANTHROPIC_API_KEY</option>
          </Select>
        </FormField>
      </Collapsible>
    </div>
  );
}

function TextField({ field, value, onChange }: { field: EnvField; value: string; onChange: (v: string) => void }) {
  const { t } = useTranslation("providers");
  const id = useId();
  return (
    <FormField label={t(field.labelKey)} htmlFor={id}>
      <Input
        id={id}
        data-testid={`env-field-${field.key}`}
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={field.placeholderKey ? t(field.placeholderKey) : undefined}
      />
    </FormField>
  );
}

function SelectField({ field, value, onChange }: { field: EnvField; value: string; onChange: (v: string) => void }) {
  const { t } = useTranslation("providers");
  const id = useId();
  const options = [
    { value: "", label: t("form.unsetOption") },
    ...(field.options?.filter((o) => o !== "").map((o) => ({ value: o, label: o })) ?? []),
  ];

  return (
    <FormField label={t(field.labelKey)} htmlFor={id}>
      <SegmentedControl
        id={id}
        data-testid={`env-field-${field.key}`}
        options={options}
        value={value}
        onChange={onChange}
        aria-label={t(field.labelKey)}
      />
    </FormField>
  );
}
