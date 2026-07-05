export const DEFAULT_LOCALE = "en";
export const SUPPORTED_LOCALES = ["en", "zh"] as const;
export type SupportedLocale = (typeof SUPPORTED_LOCALES)[number];

export const LOCALE_LABELS: Record<SupportedLocale, string> = {
  en: "English",
  zh: "简体中文",
};

export const LOCALE_NAMES: Record<SupportedLocale, string> = {
  en: "English",
  zh: "Simplified Chinese",
};

export const STORAGE_KEY = "cc-select-language";
