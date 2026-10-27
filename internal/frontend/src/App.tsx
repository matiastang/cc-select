import { useEffect, useMemo } from "react";
import i18n from "./i18n";
import { fetchBackendLanguage } from "./i18n/useLanguage";
import { LanguageSwitcher } from "./i18n/LanguageSwitcher";
import { ThemeSwitcher } from "./theme/ThemeSwitcher";
import { CheckUpdateButton } from "./features/about/CheckUpdateButton";
import { AppShell } from "./components/layout/AppShell";
import { navigate, useHashRoute } from "./navigation/router";
import { buildNavModules } from "./navigation/registry";
import { ToastProvider, TooltipProvider } from "./components/ui";
import { useDesktopMode } from "./features/desktop/useDesktopMode";
import { ProvidersModule } from "./features/providers/ProvidersModule";
import { ModeRoutesModule } from "./features/mode-routes/ModeRoutesModule";
import { KeychainCard } from "./features/security/KeychainCard";
import { DesktopSettings } from "./features/desktop/DesktopSettings";
import { GeneralModule } from "./features/general/GeneralModule";
import { AboutModule } from "./features/about/AboutModule";

// 应用壳：单一前端双形态（web/desktop）。模块注册表是导航唯一真值，
// 「桌面端」模块按 /desktop/info 探测条件注册（specs/003 FR-015 契约沿用）。
export default function App() {
  const route = useHashRoute();
  const desktop = useDesktopMode();

  useEffect(() => {
    void (async () => {
      const backend = await fetchBackendLanguage();
      if (backend) {
        await i18n.changeLanguage(backend);
      }
    })();
  }, []);

  const modules = useMemo(
    () =>
      buildNavModules({
        desktop,
        renderers: {
          providers: () => <ProvidersModule />,
          modeRoutes: () => <ModeRoutesModule />,
          security: () => (
            <div data-testid="module-security">
              <KeychainCard />
            </div>
          ),
          ...(desktop
            ? {
                desktop: () => (
                  <div data-testid="module-desktop">
                    <DesktopSettings />
                  </div>
                ),
              }
            : {}),
          general: () => <GeneralModule />,
          about: () => <AboutModule />,
        },
      }),
    [desktop],
  );

  return (
    <ToastProvider>
      <TooltipProvider>
        <AppShell modules={modules} route={route} onNavigate={navigate}>
          <CheckUpdateButton />
          <ThemeSwitcher />
          <LanguageSwitcher />
        </AppShell>
      </TooltipProvider>
    </ToastProvider>
  );
}
