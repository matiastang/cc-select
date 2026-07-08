import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ProviderDetail, IsolationMode } from "../types";
import { API_BASE } from "../constants";
import {
  Preset,
  PresetCategory,
  PresetDetail,
  CUSTOM_PRESET_ID,
  AUTH_FIELDS,
  fetchPresets,
  fetchPreset,
  applyPresetTemplate,
  placeholdersIn,
} from "../presets/presets";
import { PresetSelect } from "./PresetSelect";
import { EnvFieldEditor, EnvFieldValues } from "./EnvFieldEditor";

const PROVIDERS_API = `${API_BASE}/providers`;

function emptySettings(): Record<string, unknown> {
  return { env: {} };
}

type JsonFormProps =
  | { mode: "create"; id?: undefined; onCancel: () => void; onSaved: () => void }
  | { mode: "edit"; id: string; onCancel: () => void; onSaved: () => void };

export function JsonForm(props: JsonFormProps) {
  const { t } = useTranslation("providers");
  const isEdit = props.mode === "edit";

  const [id, setId] = useState(isEdit ? props.id : "");
  const [name, setName] = useState("");
  const [isolationMode, setIsolationMode] = useState<IsolationMode>("");

  const [presets, setPresets] = useState<Preset[]>([]);
  const [categories, setCategories] = useState<PresetCategory[]>([]);
  const [presetId, setPresetId] = useState<string>(CUSTOM_PRESET_ID);
  const [presetDetail, setPresetDetail] = useState<PresetDetail | null>(null);

  const [settings, setSettings] = useState<Record<string, unknown>>(emptySettings);
  const [jsonText, setJsonText] = useState("");
  const [jsonError, setJsonError] = useState("");
  const [lastSavedJson, setLastSavedJson] = useState("");
  const jsonEditingRef = useRef(false);

  const [loading, setLoading] = useState(isEdit);
  const [err, setErr] = useState("");

  const env = useMemo<EnvFieldValues>(() => {
    const s = settings.env;
    if (s && typeof s === "object" && !Array.isArray(s)) {
      const out: EnvFieldValues = {};
      for (const [k, v] of Object.entries(s)) {
        if (typeof v === "string") out[k] = v;
      }
      return out;
    }
    return {};
  }, [settings]);

  const authField = useMemo(() => {
    return (env._auth_field as string) || presetDetail?.authField || "ANTHROPIC_AUTH_TOKEN";
  }, [env._auth_field, presetDetail]);

  const apiFormat = useMemo(() => {
    return (env._api_format as string) || presetDetail?.apiFormat || "anthropic";
  }, [env._api_format, presetDetail]);

  // 输入框里不显示 ${API_KEY} 这类模板占位符，只显示用户真正输入过的值。
  const apiKey = useMemo(() => {
    const raw = env[authField] || "";
    if (raw !== "" && placeholdersIn(raw).length > 0) {
      return "";
    }
    return raw;
  }, [env, authField]);

  // 必填/重要字段：preset 声明的 requiredVars（排除认证字段，由当前 authField 决定） +
  // 当前 auth 字段 + Base URL。
  // 这些字段即使清空也保留在 JSON 中，并在保存时校验。
  const requiredEnvKeys = useMemo(() => {
    const set = new Set<string>(["ANTHROPIC_BASE_URL", authField]);
    if (presetDetail) {
      presetDetail.requiredVars
        .filter((k) => !(AUTH_FIELDS as readonly string[]).includes(k))
        .forEach((k) => set.add(k));
    }
    return set;
  }, [presetDetail, authField]);

  // Load presets and provider detail on mount.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const presetList = await fetchPresets();
        if (cancelled) return;
        setPresets(presetList.presets);
        setCategories(presetList.categories);

        if (isEdit) {
          const r = await fetch(`${PROVIDERS_API}/${props.id}`);
          if (!r.ok) {
            const msg = (await r.json().catch(() => ({}))).error || `${r.status}`;
            throw new Error(`${t("errors.loadFailed", { status: "" })}: ${msg}`);
          }
          const detail: ProviderDetail = await r.json();
          if (cancelled) return;

          setName(detail.name || detail.id);
          setIsolationMode((detail.isolationMode as IsolationMode) || "");

          const loadedSettings = (detail.settings || emptySettings()) as Record<string, unknown>;
          // Ensure env exists.
          if (!loadedSettings.env || typeof loadedSettings.env !== "object") {
            loadedSettings.env = {};
          }
          // Seed form meta fields from persisted provider metadata so edits keep the same auth/api format.
          const loadedEnv = loadedSettings.env as Record<string, string>;
          if (detail.apiFormat && !loadedEnv._api_format) {
            loadedEnv._api_format = detail.apiFormat;
          }
          if (detail.authField && !loadedEnv._auth_field) {
            loadedEnv._auth_field = detail.authField;
          }
          setSettings(loadedSettings);

          const initialPreset = detail.preset || CUSTOM_PRESET_ID;
          setPresetId(initialPreset);

          if (initialPreset && initialPreset !== CUSTOM_PRESET_ID) {
            const p = await fetchPreset(initialPreset).catch((e) => {
              throw new Error(`${t("errors.loadPresetFailed")}: ${e.message}`);
            });
            if (p && !cancelled) setPresetDetail(p);
          }
        }
      } catch (e) {
        if (!cancelled) setErr(String(e));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [isEdit, props.id, t]);

  // When preset changes, load detail and apply the new preset defaults.
  // We preserve the API key the user may have already typed, plus any explicit
  // advanced meta selections, but reset supplier-specific fields (baseURL, model,
  // model mapping, etc.) to the new template so JSON stays in sync.
  useEffect(() => {
    if (!presetId || presetId === CUSTOM_PRESET_ID) {
      setPresetDetail(null);
      return;
    }
    let cancelled = false;
    fetchPreset(presetId)
      .then((p) => {
        if (cancelled) return;
        setPresetDetail(p);
        setSettings((prev) => {
          const prevEnv = prev.env as EnvFieldValues;
          const prevAuthField =
            (prevEnv._auth_field as string) || presetDetail?.authField || p.authField || "ANTHROPIC_AUTH_TOKEN";
          const prevApiKey = prevEnv[prevAuthField] || "";

          const nextEnv = applyPresetTemplate(p, {});
          // Keep the already-typed API key if it looks like a real value.
          if (prevApiKey !== "" && placeholdersIn(prevApiKey).length === 0) {
            nextEnv[p.authField || "ANTHROPIC_AUTH_TOKEN"] = prevApiKey;
          }
          // Preserve explicit advanced meta choices across preset switches.
          if (prevEnv._api_format) {
            nextEnv._api_format = prevEnv._api_format;
          }
          if (prevEnv._auth_field) {
            nextEnv._auth_field = prevEnv._auth_field;
          }
          return { ...prev, env: nextEnv };
        });
      })
      .catch((e) => {
        if (!cancelled) setErr(`${t("errors.loadPresetFailed")}: ${e.message}`);
      });
    return () => {
      cancelled = true;
    };
  }, [presetId, t]);

  // Sync jsonText from settings whenever settings changes (unless user is typing JSON).
  useEffect(() => {
    if (jsonEditingRef.current) return;
    const text = JSON.stringify(settings, null, 2);
    if (text !== lastSavedJson) {
      setJsonText(text);
      setLastSavedJson(text);
      setJsonError("");
    }
  }, [settings, lastSavedJson]);

  const updateEnv = (patch: EnvFieldValues) => {
    setSettings((prev) => {
      const nextEnv = { ...(prev.env as EnvFieldValues) };
      for (const [k, v] of Object.entries(patch)) {
        if (v === "" && !requiredEnvKeys.has(k)) {
          // 非必填字段清空后从 JSON 移除；必填字段保留空字符串，便于用户感知并做校验。
          delete nextEnv[k];
        } else {
          nextEnv[k] = v;
        }
      }
      return { ...prev, env: nextEnv };
    });
  };

  const handleFieldChange = (key: string, value: string) => {
    updateEnv({ [key]: value });
  };

  const handleApiKeyChange = (value: string) => {
    updateEnv({ [authField]: value });
  };

  const handleAuthFieldChange = (newAuthField: string) => {
    setSettings((prev) => {
      const prevEnv = { ...(prev.env as EnvFieldValues) };
      const oldKey = authField;
      const value = prevEnv[oldKey] || "";
      delete prevEnv[oldKey];
      prevEnv[newAuthField] = value;
      prevEnv._auth_field = newAuthField;
      return { ...prev, env: prevEnv };
    });
  };

  const handleApiFormatChange = (value: string) => {
    updateEnv({ _api_format: value });
  };

  const handleJsonChange = (text: string) => {
    jsonEditingRef.current = true;
    setJsonText(text);
    try {
      const parsed = JSON.parse(text);
      if (typeof parsed !== "object" || Array.isArray(parsed) || parsed === null) {
        setJsonError(t("errors.jsonObject"));
        return;
      }
      setJsonError("");
      setSettings(parsed);
    } catch (e) {
      setJsonError(t("errors.jsonParse", { message: String(e) }));
    } finally {
      // Allow the next settings-derived update to run after a brief delay.
      setTimeout(() => {
        jsonEditingRef.current = false;
      }, 0);
    }
  };

  const validate = (): boolean => {
    if (!isEdit && !id.trim()) {
      setErr(t("errors.missingId"));
      return false;
    }

    // 合并 preset 模板（如果有）与当前 env 得到待校验的最终 env。
    const mergedEnv = presetDetail ? applyPresetTemplate(presetDetail, env) : { ...env };
    const required = new Set<string>(["ANTHROPIC_BASE_URL", authField]);
    if (presetDetail) {
      presetDetail.requiredVars
        .filter((k) => !(AUTH_FIELDS as readonly string[]).includes(k))
        .forEach((k) => required.add(k));
    }
    const missing = Array.from(required).filter((key) => {
      const val = mergedEnv[key] || "";
      return val === "" || placeholdersIn(val).length > 0;
    });
    if (missing.length > 0) {
      setErr(t("errors.missingRequired", { vars: missing.join(", ") }));
      return false;
    }

    if (jsonError) {
      setErr(jsonError);
      return false;
    }
    return true;
  };

  const submit = async () => {
    setErr("");
    if (!validate()) return;

    // Sanitize settings: remove internal meta fields before saving.
    const cleanEnv = { ...env };
    delete cleanEnv._api_format;
    delete cleanEnv._auth_field;
    const bodySettings = { ...settings, env: cleanEnv };

    const body = { name, settings: bodySettings, isolationMode, apiFormat, authField };
    const r = isEdit
      ? await fetch(`${PROVIDERS_API}/${props.id}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        })
      : await fetch(PROVIDERS_API, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ id: id.trim(), ...body }),
        });

    if (!r.ok) {
      const j = await r.json().catch(() => ({}));
      setErr(j.error || t("errors.saveFailed", { status: r.status }));
      return;
    }
    props.onSaved();
  };

  return (
    <div>
      <h2>{isEdit ? t("editTitle", { id: props.id }) : t("addTitle")}</h2>
      {!isEdit && (
        <>
          <label>{t("form.idLabel")}</label>
          <input
            data-testid="provider-id-input"
            value={id}
            onChange={(e) => setId(e.target.value)}
            placeholder={t("form.idPlaceholder")}
          />
        </>
      )}
      <label>{t("form.nameLabel")}</label>
      <input
        data-testid="provider-name-input"
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder={t("form.namePlaceholder")}
      />
      <label>{t("form.modeLabel")}</label>
      <select
        data-testid="provider-mode-select"
        value={isolationMode}
        onChange={(e) => setIsolationMode(e.target.value as IsolationMode)}
      >
        <option value="">{t("mode.inherit")}</option>
        <option value="settings-only">{t("mode.settingsOnly")}</option>
        <option value="full">{t("mode.full")}</option>
      </select>

      {loading ? (
        <p className="muted">{t("form.loadingRealConfig")}</p>
      ) : (
        <>
          <PresetSelect
            presets={presets}
            categories={categories}
            value={presetId}
            onChange={setPresetId}
            label={t("form.presetLabel")}
          />

          <EnvFieldEditor
            preset={presetDetail}
            values={env}
            onChange={handleFieldChange}
            apiKey={apiKey}
            onApiKeyChange={handleApiKeyChange}
            apiFormat={apiFormat}
            onApiFormatChange={handleApiFormatChange}
            authField={authField}
            onAuthFieldChange={handleAuthFieldChange}
          />

          <hr style={{ margin: "1.5rem 0", borderColor: "var(--border)" }} />

          <label>{t("form.jsonLabel")}</label>
          <textarea
            data-testid="provider-json-textarea"
            value={jsonText}
            onChange={(e) => handleJsonChange(e.target.value)}
            onBlur={() => {
              jsonEditingRef.current = false;
              // Normalize valid JSON on blur.
              if (!jsonError) {
                setJsonText(JSON.stringify(settings, null, 2));
              }
            }}
            spellCheck={false}
            rows={12}
            style={{
              width: "100%",
              fontFamily: "monospace",
              fontSize: "0.85rem",
              padding: "0.5rem",
              border: "1px solid var(--border)",
              borderRadius: 6,
              background: "var(--bg)",
              color: "var(--text)",
            }}
          />
          {jsonError && (
            <div style={{ color: "var(--danger)", fontSize: "0.85rem", marginTop: "0.25rem" }}>
              {jsonError}
            </div>
          )}
        </>
      )}

      {err && (
        <div data-testid="provider-form-error" className="muted" style={{ color: "var(--danger)", margin: "0.5rem 0" }}>
          {err}
        </div>
      )}
      <div style={{ marginTop: "1rem" }}>
        <button data-testid="provider-save-button" onClick={submit} disabled={loading}>
          {t("save", { ns: "common" })}
        </button>{" "}
        <button data-testid="provider-cancel-button" className="secondary" onClick={props.onCancel}>
          {t("cancel", { ns: "common" })}
        </button>
      </div>
    </div>
  );
}
