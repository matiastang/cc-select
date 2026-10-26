import React, { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { updateDocumentLang, updateDocumentTitle } from "../../i18n/utils";
import type { NavModule } from "../../navigation/registry";
import { cn } from "../../lib/cn";
import { useMediaQuery } from "../../lib/useMediaQuery";

type AppShellProps = {
  modules: NavModule[];
  route: string;
  onNavigate: (route: string) => void;
  children?: React.ReactNode; // 顶栏右侧控件槽（主题/语言/更新）
};

// 布局壳（contracts/frontend §2/§4）：≥880px 侧边栏导航，<880px 顶部 tab；
// 两种形态由同一模块注册表渲染。文档标题/语言随 i18n 联动（承自旧 Header）。
export function AppShell({ modules, route, onNavigate, children }: AppShellProps) {
  const { t, i18n } = useTranslation();
  const wide = useMediaQuery("(min-width: 880px)");

  useEffect(() => {
    updateDocumentLang(i18n.language);
    updateDocumentTitle(t("appTitle"));
  }, [i18n.language, t]);

  const active = modules.find((m) => m.route === route) ?? modules[0];

  const navItem = (m: NavModule, variant: "sidebar" | "tab") => {
    const Icon = m.icon;
    const isActive = m.route === active.route;
    return (
      <button
        key={m.route}
        type="button"
        data-testid={m.navTestId}
        aria-current={isActive ? "page" : undefined}
        onClick={() => onNavigate(m.route)}
        className={cn(
          "flex items-center gap-2 text-sm font-medium transition-colors rounded-md",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50",
          variant === "sidebar" ? "px-3 py-2 justify-start" : "px-3 py-1.5",
          isActive ? "bg-notice text-accent" : "text-muted hover:bg-bg hover:text-text",
        )}
      >
        <Icon size={16} />
        <span>{t(m.titleKey)}</span>
      </button>
    );
  };

  return (
    <div className="ui-app-shell min-h-screen bg-bg text-text">
      <header className="sticky top-0 z-30 border-b border-border bg-card">
        <div className="mx-auto flex max-w-5xl items-center justify-between gap-3 px-4 py-3">
          <h1 className="truncate text-lg font-semibold">{t("appTitle")}</h1>
          <div data-testid="header-controls" className="flex items-center gap-1.5">
            {children}
          </div>
        </div>
      </header>
      <div className="mx-auto flex max-w-5xl flex-col gap-4 px-4 py-5 wide:flex-row wide:gap-6">
        {wide ? (
          <nav
            aria-label={t("nav.label")}
            data-testid="nav-sidebar"
            className="flex shrink-0 flex-col gap-1 wide:w-48"
          >
            {modules.map((m) => navItem(m, "sidebar"))}
          </nav>
        ) : (
          <nav aria-label={t("nav.label")} data-testid="nav-tabs" className="flex flex-wrap gap-1">
            {modules.map((m) => navItem(m, "tab"))}
          </nav>
        )}
        <main className="flex min-w-0 flex-1 flex-col gap-4" data-testid="module-outlet">
          {active?.render()}
        </main>
      </div>
    </div>
  );
}
