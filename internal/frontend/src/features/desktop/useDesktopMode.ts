import { useEffect, useState } from "react";

// 桌面能力探测（specs/003 contracts/desktop-api §3.1）：/desktop/info 在 web 模式
// 404。App 壳据此决定是否注册「桌面端」导航模块（FR-001/FR-002 条件渲染契约）。
export function useDesktopMode(): boolean {
  const [desktop, setDesktop] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const r = await fetch("./api/v1/desktop/info");
        if (!cancelled) setDesktop(r.ok);
      } catch {
        if (!cancelled) setDesktop(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return desktop;
}
