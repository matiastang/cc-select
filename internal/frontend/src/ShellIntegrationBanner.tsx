import { useEffect, useState } from "react";

// ShellIntegrationBanner：首访检测 shell 集成是否已装，未装则提供一键安装。
// 自包含状态机；平台/shell 判断全在后端——前端只按 Status/InstallResult 渲染。
// 已装或不支持的 shell（fish 等）→ 隐藏，零干扰。
export function ShellIntegrationBanner() {
  const [state, setState] = useState<
    "loading" | "needed" | "installing" | "done" | "manual" | "unsupported" | "hidden"
  >("loading");
  const [shell, setShell] = useState("");
  const [legacy, setLegacy] = useState(false);
  const [doneMsg, setDoneMsg] = useState("");
  const [manual, setManual] = useState<{ snippet: string; message: string } | null>(null);
  const [err, setErr] = useState("");

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const r = await fetch("/api/v1/shell-integration");
        if (!r.ok) return;
        const d = await r.json();
        if (cancelled) return;
        setShell(d.shell || "");
        setLegacy(d.legacy === true);
        if (!d.supported) {
          setState("unsupported");
        } else if (d.installed) {
          setState("hidden");
        } else {
          setState("needed");
        }
      } catch {
        setState("hidden"); // 静默失败，不影响主配置页。
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const install = async () => {
    setState("installing");
    setErr("");
    try {
      const r = await fetch("/api/v1/shell-integration/install", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ shell }),
      });
      const d = await r.json().catch(() => ({}));
      if (!r.ok) {
        setErr(d.error || "安装失败");
        setState("needed");
        return;
      }
      if (d.action === "manual") {
        setManual({ snippet: d.snippet || "", message: d.message || "" });
        setState("manual");
      } else {
        setDoneMsg(d.message || "已安装");
        setState("done");
      }
    } catch (e) {
      setErr(String(e));
      setState("needed");
    }
  };

  if (state === "loading" || state === "hidden") return null;

  if (state === "done") {
    return <div className="notice">✅ {doneMsg}</div>;
  }

  if (state === "manual") {
    return (
      <div className="notice">
        <strong>需要手动完成 shell 集成</strong>
        <div className="muted">{manual?.message}</div>
        <textarea
          readOnly
          value={manual?.snippet || ""}
          spellCheck={false}
          rows={10}
          style={{
            width: "100%",
            fontFamily: "monospace",
            fontSize: "0.85rem",
            marginTop: "0.5rem",
            padding: "0.5rem",
            border: "1px solid var(--border)",
            borderRadius: 6,
            background: "var(--bg)",
            color: "var(--text)",
          }}
        />
      </div>
    );
  }

  if (state === "unsupported") {
    return (
      <div className="notice">
        <strong>当前 shell{shell ? `（${shell}）` : ""}暂不支持一键安装</strong>
        <div className="muted">
          请使用 zsh / bash / PowerShell，或在终端手动执行 <code>cc-select init</code>。
        </div>
      </div>
    );
  }

  // needed / installing
  return (
    <div className="notice">
      <strong>
        {legacy ? "检测到旧版 shell 集成，建议升级" : "检测到尚未安装 shell 集成"}
        {shell ? `（${shell}）` : ""}
      </strong>
      <div className="muted">
        安装后才能在终端用 <code>ccs use &lt;id&gt;</code> 切换 provider。
      </div>
      {err && (
        <div className="muted" style={{ color: "var(--danger)", marginTop: "0.5rem" }}>
          {err}
        </div>
      )}
      <div style={{ marginTop: "0.5rem" }}>
        <button onClick={install} disabled={state === "installing"}>
          {state === "installing" ? "安装中…" : "一键安装"}
        </button>
      </div>
    </div>
  );
}
