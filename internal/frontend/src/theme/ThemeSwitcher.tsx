import { useTranslation } from "react-i18next";
import { Icon, IconName } from "../components/ui";
import { Theme, useTheme } from "./useTheme";

const THEMES: Theme[] = ["system", "light", "dark"];

const THEME_ICONS: Record<Theme, IconName> = {
  system: "monitor",
  light: "sun",
  dark: "moon",
};

export function ThemeSwitcher() {
  const { t } = useTranslation("common");
  const [theme, setTheme] = useTheme();

  return (
    <div className="theme-switcher" role="group" aria-label={t("theme.label")}>
      {THEMES.map((value) => (
        <button
          key={value}
          type="button"
          className={theme === value ? "active" : ""}
          onClick={() => setTheme(value)}
          aria-pressed={theme === value}
          data-testid={`theme-${value}`}
        >
          <Icon name={THEME_ICONS[value]} size={14} />
          <span>{t(`theme.${value}`)}</span>
        </button>
      ))}
    </div>
  );
}
