import { useCallback, useEffect, useState } from "react";

// 极简 hash 路由（contracts/frontend §4）：hashchange 驱动，未知路由归一默认。
// 不引入 react-router——模块集合固定且小（YAGNI）。
export const DEFAULT_ROUTE = "#/providers";

const KNOWN_ROUTES = new Set([
  "#/providers",
  "#/mode-routes",
  "#/security",
  "#/desktop",
  "#/general",
  "#/about",
]);

export function parseHash(hash: string): string {
  const normalized = hash.replace(/\/+$/, "") || "#/";
  return KNOWN_ROUTES.has(normalized) ? normalized : DEFAULT_ROUTE;
}

export function navigate(route: string): void {
  window.location.hash = route;
}

export function useHashRoute(): string {
  const [route, setRoute] = useState(() => parseHash(window.location.hash));

  useEffect(() => {
    const onChange = () => setRoute(parseHash(window.location.hash));
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);

  const go = useCallback((next: string) => {
    navigate(next);
  }, []);

  // 返回当前路由不足以同时暴露 go；分开导出 navigate 已够用，
  // hook 只回路由字符串以保持最小面。
  void go;
  return route;
}
