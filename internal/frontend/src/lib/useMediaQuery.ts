import { useEffect, useState } from "react";

// jsdom 无真实媒体查询（matches 恒 false→顶部 tab 形态）；浏览器/WebView 正常响应。
// 返回 false 时调用方渲染窄屏形态——单 DOM 导航避免 testid 重复（contracts/frontend §5）。
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => {
    if (typeof window.matchMedia !== "function") return true;
    return window.matchMedia(query).matches;
  });

  useEffect(() => {
    if (typeof window.matchMedia !== "function") return;
    const mql = window.matchMedia(query);
    const onChange = (e: MediaQueryListEvent) => setMatches(e.matches);
    setMatches(mql.matches);
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }, [query]);

  return matches;
}
