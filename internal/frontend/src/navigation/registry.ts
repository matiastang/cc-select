import type { LucideIcon } from "lucide-react";
import { Boxes, Info, Monitor, Palette, Route, ShieldCheck } from "lucide-react";

// 模块注册表是导航唯一真值（contracts/frontend §4）：侧边栏与顶部 tab 同源渲染。
// render 为惰性字段——App 壳按当前路由挂载对应模块，避免全量常驻。
export interface NavModule {
  route: string;
  titleKey: string; // common 命名空间 i18n key
  icon: LucideIcon;
  render: () => JSX.Element;
  desktopOnly?: boolean;
  navTestId: string; // nav-{route} 契约（contracts/frontend §5）
}

interface BuildOptions {
  desktop: boolean;
  renderers: ModuleRenderers;
}

export interface ModuleRenderers {
  providers: () => JSX.Element;
  modeRoutes: () => JSX.Element;
  security: () => JSX.Element;
  desktop?: () => JSX.Element;
  general: () => JSX.Element;
  about: () => JSX.Element;
}

function module(
  route: string,
  titleKey: string,
  icon: LucideIcon,
  render: () => JSX.Element,
  desktopOnly = false,
): NavModule {
  return {
    route,
    titleKey,
    icon,
    render,
    desktopOnly,
    navTestId: `nav-${route.replace("#/", "")}`,
  };
}

export function buildNavModules({ desktop, renderers }: BuildOptions): NavModule[] {
  const mods: NavModule[] = [
    module("#/providers", "nav.providers", Boxes, renderers.providers),
    module("#/mode-routes", "nav.modeRoutes", Route, renderers.modeRoutes),
    module("#/security", "nav.security", ShieldCheck, renderers.security),
  ];
  if (desktop && renderers.desktop) {
    mods.push(module("#/desktop", "nav.desktop", Monitor, renderers.desktop, true));
  }
  mods.push(module("#/general", "nav.general", Palette, renderers.general));
  mods.push(module("#/about", "nav.about", Info, renderers.about));
  return mods;
}
