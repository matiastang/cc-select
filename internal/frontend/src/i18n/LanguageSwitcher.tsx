import { SUPPORTED_LOCALES, LOCALE_LABELS, SupportedLocale } from "./config";
import { useLanguage } from "./useLanguage";

export function LanguageSwitcher() {
  const { language, changeLanguage } = useLanguage();

  return (
    <div className="language-switcher" role="group" aria-label="Language switcher">
      {SUPPORTED_LOCALES.map((lng) => (
        <button
          key={lng}
          className={language === lng ? "active" : ""}
          onClick={() => changeLanguage(lng as SupportedLocale)}
          aria-pressed={language === lng}
        >
          {LOCALE_LABELS[lng]}
        </button>
      ))}
    </div>
  );
}
