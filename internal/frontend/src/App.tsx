import { useEffect, useState } from "react";
import { ShellIntegrationBanner } from "./ShellIntegrationBanner";

type Provider = {
  id: string;
  name: string;
  env: Record<string, string>;
  hasKey: boolean;
  varKeys: string[];
  isolationMode: string;
};

// providerDetail 对应后端 GET /providers/{id}：含磁盘 settings.json 原文（明文）。
type ProviderDetail = {
  id: string;
  name: string;
  settings: unknown;
  isolationMode: string;
};

type IsolationMode = "" | "settings-only" | "full";

const API = "/api/v1/providers";
const MODE_API = "/api/v1/mode";

// 内置官方 provider 的 id。它使用系统默认配置（~/.claude），不建独立 profile，
// 故不可删除、也不可自定义 settings——切到它等于 unset CLAUDE_CONFIG_DIR。
const OFFICIAL_ID = "claude-official";

// 新建 provider 时预填的 settings 模板：完整 settings.json，不止 env。
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

const MODE_LABELS: Record<IsolationMode, string> = {
  "": "继承全局",
  "settings-only": "仅 settings.json 隔离（共享历史/插件）",
  full: "整目录隔离（完全独立）",
};

export default function App() {
  const [providers, setProviders] = useState<Record<string, Provider>>({});
  const [globalMode, setGlobalMode] = useState<IsolationMode>("settings-only");
  const [globalModeLoading, setGlobalModeLoading] = useState(true);
  const [editing, setEditing] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string>("");

  const refresh = async () => {
    try {
      const r = await fetch(API);
      const data = await r.json();
      setProviders(data.providers || {});
      setError("");
    } catch (e) {
      setError(String(e));
    }
  };

  const loadGlobalMode = async () => {
    try {
      const r = await fetch(MODE_API);
      const data = await r.json();
      setGlobalMode(data.isolationMode || "settings-only");
    } catch (e) {
      setError(String(e));
    } finally {
      setGlobalModeLoading(false);
    }
  };

  useEffect(() => {
    refresh();
    loadGlobalMode();
  }, []);

  const saveGlobalMode = async (mode: IsolationMode) => {
    setGlobalMode(mode);
    try {
      const r = await fetch(MODE_API, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ isolationMode: mode }),
      });
      if (!r.ok) {
        const j = await r.json().catch(() => ({}));
        setError(j.error || "保存全局模式失败");
        await loadGlobalMode(); // 回显服务端真实值
      } else {
        setError("");
      }
    } catch (e) {
      setError(String(e));
      await loadGlobalMode();
    }
  };

  const remove = async (id: string) => {
    if (!confirm(`删除 provider ${id}？`)) return;
    const r = await fetch(`${API}/${id}`, { method: "DELETE" });
    if (!r.ok) setError((await r.json()).error || "删除失败");
    refresh();
  };

  return (
    <div className="container">
      <h1>cc-select 配置</h1>
      <p className="muted">管理各 AI 服务商配置。切换请在终端用 <code>ccs use &lt;id&gt;</code>。</p>

      <ShellIntegrationBanner />

      <div className="notice">
        配置以完整 <code>settings.json</code> 形式编辑（不止 <code>env</code>，<code>permissions</code>、<code>model</code> 等均可）。
        在此处修改是改“模板”，<strong>已在运行的终端不会自动变化</strong>，需在对应终端重新执行 <code>ccs use &lt;id&gt;</code> 才生效。
      </div>

      {error && <div className="notice" style={{ background: "rgba(209,36,47,0.1)", borderLeftColor: "var(--danger)" }}>{error}</div>}

      <div className="card">
        <h2 style={{ marginTop: 0 }}>全局隔离模式</h2>
        <p className="muted">默认对所有 provider 生效；单个 provider 可单独覆盖。</p>
        {globalModeLoading ? (
          <p className="muted">加载中…</p>
        ) : (
          <select
            value={globalMode}
            onChange={(e) => saveGlobalMode(e.target.value as IsolationMode)}
            style={{ width: "100%", padding: "0.5rem", fontSize: "0.95rem" }}
          >
            <option value="settings-only">{MODE_LABELS["settings-only"]}</option>
            <option value="full">{MODE_LABELS["full"]}</option>
          </select>
        )}
      </div>

      {Object.values(providers)
        .sort((a, b) => a.id.localeCompare(b.id))
        .map((p) => (
          <div className="card" key={p.id}>
            {editing === p.id ? (
              <JsonForm
                mode="edit"
                id={p.id}
                onCancel={() => setEditing(null)}
                onSaved={() => {
                  setEditing(null);
                  refresh();
                }}
              />
            ) : (
              <div className="row">
                <div>
                  <strong>{p.name || p.id}</strong>{" "}
                  <span className="muted">({p.id})</span>{" "}
                  {p.hasKey && <span className="badge">已配置 key</span>}
                  <div className="muted">
                    {p.id === OFFICIAL_ID ? (
                      "使用系统默认配置（~/.claude），不可自定义"
                    ) : (
                      <>
                        {(p.env.ANTHROPIC_BASE_URL && `URL: ${p.env.ANTHROPIC_BASE_URL}`) || "（无 base url）"}
                        {(p.env.ANTHROPIC_MODEL && ` · 模型: ${p.env.ANTHROPIC_MODEL}`) || ""}
                        {" · "}
                        模式: {p.isolationMode ? MODE_LABELS[p.isolationMode as IsolationMode] : "继承全局"}
                      </>
                    )}
                  </div>
                </div>
                <div>
                  {p.id !== OFFICIAL_ID && (
                    <>
                      <button className="secondary" onClick={() => setEditing(p.id)}>编辑</button>{" "}
                      <button className="danger" onClick={() => remove(p.id)}>
                        删除
                      </button>
                    </>
                  )}
                </div>
              </div>
            )}
          </div>
        ))}

      <div className="card">
        {creating ? (
          <JsonForm
            mode="create"
            onCancel={() => setCreating(false)}
            onSaved={() => {
              setCreating(false);
              refresh();
            }}
          />
        ) : (
          <div className="row">
            <button onClick={() => setCreating(true)}>+ 添加 provider</button>
          </div>
        )}
      </div>
    </div>
  );
}

type JsonFormProps =
  | { mode: "create"; id?: undefined; onCancel: () => void; onSaved: () => void }
  | { mode: "edit"; id: string; onCancel: () => void; onSaved: () => void };

// JsonForm：添加与编辑的唯一表单。两者都只支持粘贴/编辑完整 settings.json。
// 编辑模式挂载时从 GET /providers/{id} 现读磁盘真实内容——即便用户手改过文件也如实反映。
function JsonForm(props: JsonFormProps) {
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
        if (!r.ok) throw new Error((await r.json()).error || `加载失败 (${r.status})`);
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
  }, [isEdit, props.id]);

  const submit = async () => {
    setErr("");
    if (!isEdit && !id.trim()) {
      setErr("缺少 id（短标识，如 glm）");
      return;
    }
    // 前端先校验 JSON，给出更友好的报错（后端也会再校验一次）。
    let settings: unknown;
    try {
      settings = JSON.parse(jsonText);
    } catch (e) {
      setErr("JSON 解析失败：" + e);
      return;
    }
    if (settings === null || typeof settings !== "object" || Array.isArray(settings)) {
      setErr("settings 必须是 JSON 对象");
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
      setErr(j.error || `保存失败 (${r.status})`);
      return;
    }
    props.onSaved();
  };

  return (
    <div>
      <h2>{isEdit ? `编辑 ${props.id}` : "添加 provider"}</h2>
      {!isEdit && (
        <>
          <label>ID（短标识，如 glm）</label>
          <input value={id} onChange={(e) => setId(e.target.value)} placeholder="glm" />
        </>
      )}
      <label>展示名（可留空，默认用 id）</label>
      <input value={name} onChange={(e) => setName(e.target.value)} placeholder="智谱 GLM" />
      <label>隔离模式</label>
      <select
        value={isolationMode}
        onChange={(e) => setIsolationMode(e.target.value as IsolationMode)}
        style={{ width: "100%", padding: "0.5rem", marginBottom: "1rem", fontSize: "0.95rem" }}
      >
        <option value="">{MODE_LABELS[""]}</option>
        <option value="settings-only">{MODE_LABELS["settings-only"]}</option>
        <option value="full">{MODE_LABELS["full"]}</option>
      </select>
      <label>settings.json（完整内容；env、permissions、model 等均可）</label>
      {loading ? (
        <p className="muted">加载磁盘真实配置中…</p>
      ) : (
        <textarea
          value={jsonText}
          onChange={(e) => setJsonText(e.target.value)}
          spellCheck={false}
          rows={14}
          style={{ width: "100%", fontFamily: "monospace", fontSize: "0.85rem", padding: "0.5rem", border: "1px solid var(--border)", borderRadius: 6, background: "var(--bg)", color: "var(--text)" }}
        />
      )}
      {err && <div className="muted" style={{ color: "var(--danger)", margin: "0.5rem 0" }}>{err}</div>}
      <div style={{ marginTop: "1rem" }}>
        <button onClick={submit} disabled={loading}>保存</button>{" "}
        <button className="secondary" onClick={props.onCancel}>取消</button>
      </div>
    </div>
  );
}
