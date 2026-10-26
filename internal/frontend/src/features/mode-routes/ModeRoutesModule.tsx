import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { GlobalModeCard } from "./GlobalModeCard";
import { RoutesPanel } from "./RoutesPanel";
import { IsolationMode } from "../../types";
import { API_BASE } from "../../constants";

const MODE_API = `${API_BASE}/mode`;

// 「模式与路由」模块：全局隔离模式切换（承自旧 App 的状态逻辑）+ Mode P 路由面板。
export function ModeRoutesModule() {
  const { t } = useTranslation("providers");
  const [globalMode, setGlobalMode] = useState<IsolationMode>("settings-only");
  const [globalModeLoading, setGlobalModeLoading] = useState(true);

  useEffect(() => {
    void (async () => {
      try {
        const r = await fetch(MODE_API);
        const data = await r.json();
        setGlobalMode(data.isolationMode || "settings-only");
      } catch {
        // 模式加载失败时保持默认值，卡片自身不阻断模块渲染
      } finally {
        setGlobalModeLoading(false);
      }
    })();
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
        throw new Error(j.error || t("errors.saveGlobalModeFailed"));
      }
    } catch {
      // 失败回滚到服务端真值
      try {
        const r = await fetch(MODE_API);
        const data = await r.json();
        setGlobalMode(data.isolationMode || "settings-only");
      } catch {
        /* 网络不可达：保持乐观值，下次进入模块自愈 */
      }
    }
  };

  return (
    <div data-testid="module-mode-routes" className="flex flex-col gap-4">
      <GlobalModeCard mode={globalMode} loading={globalModeLoading} onChange={saveGlobalMode} />
      <RoutesPanel />
    </div>
  );
}
