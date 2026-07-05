import { DEFAULT_LOCALE, SUPPORTED_LOCALES, SupportedLocale } from "./config";

export function mapDetectedLanguage(language: string): SupportedLocale {
  const normalized = language.toLowerCase();
  if (normalized.startsWith("zh")) {
    return "zh";
  }
  if (normalized.startsWith("en")) {
    return "en";
  }
  return DEFAULT_LOCALE;
}

export function isSupportedLocale(language: string): language is SupportedLocale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(language);
}
