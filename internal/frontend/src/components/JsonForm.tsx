import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ProviderDetail, IsolationMode } from "../types";
import { API_BASE } from "../constants";

const API = `${API_BASE}/providers`;

// Pre-filled settings template when creating a new provider: full settings.json, not just env.
const NEW_TEMPLATE = JSON.stringify(
  {
    env: {
      ANTHROPIC_BASE_URL: "https://open.bigmodel.cn/api/anthropic",
      ANTHROPIC_AUTH_TOKEN: "sk-...",
      ANTHROPIC_MODEL: "glm-4.6",
    },
  },
  null,
  2,
);

type JsonFormProps =
  | { mode: "create"; id?: undefined; onCancel: () => void; onSaved: () => void }
  | { mode: "edit"; id: string; onCancel: () => void; onSaved: () => void };

// JsonForm: shared create/edit form. Both only support pasting/editing the full settings.json.
// Edit mode reads the real disk content on mount via GET /providers/{id}.
export function JsonForm(props: JsonFormProps) {
  const { t, i18n } = useTranslation("providers");
  const isEdit = props.mode === "edit";
  const [id, setId] = useState(isEdit ? props.id : "");
  const [name, setName] = useState("");
  const [jsonText, setJsonText] = useState(isEdit ? "" : NEW_TEMPLATE);
  const [isolationMode, setIsolationMode] = useState<IsolationMode>("");
  const [loading, setLoading] = useState(isEdit);
  const [err, setErr] = useState("");

  useEffect(() => {
    if (!isEdit) return;
    let cancelled = false;
    (async () => {
      try {
        const r = await fetch(`${API}/${props.id}`);
        if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || i18n.t("errors.loadFailed", { status: r.status, ns: "providers" }));
        const detail: ProviderDetail = await r.json();
        if (cancelled) return;
        setName(detail.name || "");
        setJsonText(JSON.stringify(detail.settings ?? {}, null, 2));
        setIsolationMode((detail.isolationMode as IsolationMode) || "");
      } catch (e) {
        if (!cancelled) setErr(String(e));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [isEdit, props.id, i18n]);

  const submit = async () => {
    setErr("");
    if (!isEdit && !id.trim()) {
      setErr(t("errors.missingId"));
      return;
    }
    // Client-side JSON validation for friendlier errors (server also validates).
    let settings: unknown;
    try {
      settings = JSON.parse(jsonText);
    } catch (e) {
      setErr(t("errors.jsonParse", { message: String(e) }));
      return;
    }
    if (settings === null || typeof settings !== "object" || Array.isArray(settings)) {
      setErr(t("errors.jsonObject"));
      return;
    }

    const body = { name, settings, isolationMode };
    const r = isEdit
      ? await fetch(`${API}/${props.id}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        })
      : await fetch(API, {
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
        style={{ width: "100%", padding: "0.5rem", marginBottom: "1rem", fontSize: "0.95rem" }}
      >
        <option value="">{t("mode.inherit")}</option>
        <option value="settings-only">{t("mode.settingsOnly")}</option>
        <option value="full">{t("mode.full")}</option>
      </select>
      <label>{t("form.jsonLabel")}</label>
      {loading ? (
        <p className="muted">{t("form.loadingRealConfig")}</p>
      ) : (
        <textarea
          data-testid="provider-json-textarea"
          value={jsonText}
          onChange={(e) => setJsonText(e.target.value)}
          spellCheck={false}
          rows={14}
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
